package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CBookShu/kd48/gateway/internal/bootstrap"
	"github.com/CBookShu/kd48/gateway/internal/ws"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/metrics"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/rediskit"
	"github.com/CBookShu/kd48/pkg/registry"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcresolver "google.golang.org/grpc/resolver"
)

func main() {
	ctx := context.Background()

	c, err := conf.Load("./config.yaml")
	if err != nil {
		panic(err)
	}

	logPath := filepath.Join(c.Log.FilePath, "gateway.log")
	handler := logzap.New(c.Log.Level, logPath)
	slog.SetDefault(slog.New(handler))

	shutdown, err := otelkit.InitTracer(c.Server.Name + "-gateway")
	if err != nil {
		slog.Error("Init otel failed", "error", err)
		panic(err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("OTel shutdown error", "error", err)
		}
	}()

	rdb, err := rediskit.NewClient(c.Redis)
	if err != nil {
		panic(err)
	}
	defer rdb.Close()

	etcdCli, err := registry.NewClient(c.Etcd)
	if err != nil {
		panic(err)
	}
	defer etcdCli.Close()

	etcdResolverBuilder, err := resolver.NewBuilder(etcdCli)
	if err != nil {
		panic(err)
	}
	grpcresolver.Register(etcdResolverBuilder)

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	atomicRT := ws.NewAtomicRouter()
	mgr := bootstrap.NewManager(etcdCli, atomicRT, c.Gateway.MetaServiceTypesPrefix, c.Gateway.MetaGatewayRoutesPrefix, dialOpts)
	if err := mgr.Bootstrap(ctx); err != nil {
		slog.Error("gateway meta bootstrap failed", "error", err)
		os.Exit(1)
	}

	metaCtx, metaCancel := context.WithCancel(ctx)
	defer metaCancel()
	go mgr.Run(metaCtx)
	defer mgr.Close()

	// 初始化连接管理器
	// 配置说明：
	// - Interval: 心跳检测间隔（客户端应在此间隔内发送Ping）
	// - Timeout: 心跳超时（超过此时间未收到响应视为可能断开）
	// - MaxMissed: 最大允许丢失心跳次数（连续超时达到此值则断开连接）
	// - IdleTimeout: 空闲超时（无任何活动达到此时间则断开连接，0表示不启用）
	heartbeatConfig := ws.HeartbeatConfig{
		Interval:    30 * time.Second, // 心跳间隔30秒
		Timeout:     45 * time.Second, // 超时45秒（比间隔长，允许网络延迟）
		MaxMissed:   3,                // 允许连续3次超时
		IdleTimeout: 30 * time.Minute, // 30分钟无活动断开
	}
	connManager := ws.NewConnectionManager(heartbeatConfig)

	// 启动连接管理器的后台检查
	connManagerCtx, connManagerCancel := context.WithCancel(ctx)
	defer connManagerCancel()
	go connManager.Start(connManagerCtx)

	// 启动 Session 失效订阅器
	sessionSubscriber := ws.NewSessionInvalidationSubscriber(rdb, connManager, ws.SessionInvalidateChannel)
	subscriberCtx, subscriberCancel := context.WithCancel(ctx)
	defer subscriberCancel()
	go sessionSubscriber.Start(subscriberCtx)
	slog.Info("Session invalidate subscriber started")

	tracer := otel.Tracer("github.com/CBookShu/kd48/gateway")

	wsHandler := ws.NewHandler(tracer, atomicRT, connManager)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Get("/metrics", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("# Metrics endpoint placeholder")
	})

	app.Use(metrics.FiberMiddleware("gateway"))

	SetupRoutes(app, wsHandler, c.Gateway.StaticDir)

	go func() {
		slog.Info("Gateway Fiber WS server listening", "port", c.Gateway.Port)
		if err := app.Listen(fmt.Sprintf(":%d", c.Gateway.Port)); err != nil {
			panic(err)
		}
	}()

	slog.Info("Server is running, press Ctrl+C to stop...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	metaCancel()
	connManagerCancel() // 停止连接管理器
	connManager.Stop()  // 等待连接管理器完全停止
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		slog.Error("Fiber shutdown error", "error", err)
	}
}
