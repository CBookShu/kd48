package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/gateway/internal/ws"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/rediskit"
	"github.com/CBookShu/kd48/pkg/registry"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcresolver "google.golang.org/grpc/resolver"
)

func main() {
	c, err := conf.Load("./config.yaml")
	if err != nil {
		panic(err)
	}

	// 1. 初始化日志
	handler := logzap.New(c.Log.Level)
	slog.SetDefault(slog.New(handler))

	// 2. 初始化 OTel
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

	// 1. gRPC 连接
	etcdResolverBuilder, err := resolver.NewBuilder(etcdCli)
	if err != nil {
		panic(err)
	}
	grpcresolver.Register(etcdResolverBuilder)
	conn, err := grpc.Dial(
		"etcd:///kd48/user-service",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // 轮询负载均衡
	)
	if err != nil {
		slog.Error("gRPC dial failed", "error", err)
		panic(err)
	}
	defer conn.Close()

	userClient := userv1.NewUserServiceClient(conn)
	slog.Info("Gateway connected to User Service cluster via Etcd")

	// 2. 初始化 Fiber App
	app := fiber.New(fiber.Config{
		// 关闭 Fiber 启动时的 ASCII Banner，保持日志整洁
		DisableStartupMessage: true,
	})

	// 3. 注册路由
	// 健康检查
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// WebSocket 路由
	// 必须先经过 IsWebSocketUpgrade 中间件拦截非 WS 请求
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	wsHandler := ws.NewHandler(userClient)
	// websocket.New 会自动完成握手，并将 conn 存入 c.Locals("websocket")
	app.Get("/ws", websocket.New(wsHandler.ServeWS))

	// 4. 启动服务
	go func() {
		slog.Info("Gateway Fiber WS server listening", "port", c.Gateway.Port)
		if err := app.Listen(fmt.Sprintf(":%d", c.Gateway.Port)); err != nil {
			panic(err)
		}
	}()

	// 阻塞等待退出信号
	slog.Info("Server is running, press Ctrl+C to stop...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")
}
