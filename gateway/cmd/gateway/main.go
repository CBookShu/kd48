package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/gateway/internal/ws" // 🚨 新增：引入 ws 包用于构建路由
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/rediskit"
	"github.com/CBookShu/kd48/pkg/registry"

	"github.com/gofiber/fiber/v2"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
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

	// 3. gRPC 连接池
	etcdResolverBuilder, err := resolver.NewBuilder(etcdCli)
	if err != nil {
		panic(err)
	}
	grpcresolver.Register(etcdResolverBuilder)

	conn, err := grpc.Dial(
		"etcd:///kd48/user-service",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("gRPC dial failed", "error", err)
		panic(err)
	}
	defer conn.Close()

	userClient := userv1.NewUserServiceClient(conn)
	slog.Info("Gateway connected to User Service cluster via Etcd")

	tracer := otel.Tracer("github.com/CBookShu/kd48/gateway")

	// ==========================================
	// 🚨 核心重构：动态路由表组装 (显式契约注入)
	// ==========================================
	wsRouter := ws.NewWsRouter()

	// 将 gRPC 方法包装并注册到网关路由 (一行代码接入一个接口)
	wsRouter.Register("/user.v1.UserService/Login", ws.WrapUnary(userClient.Login))

	// 💡 后续如果有 Room Service，只需在此追加：
	// roomClient := roomv1.NewRoomServiceClient(roomConn)
	// wsRouter.Register("/room.v1.RoomService/CreateRoom", ws.WrapUnary(roomClient.CreateRoom))

	// 将路由表注入给网关 Handler
	wsHandler := ws.NewHandler(tracer, wsRouter)
	// ==========================================

	// 4. 初始化 Fiber App
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// 5. 挂载路由 (不再需要把具体的 userClient 传给路由层)
	SetupRoutes(app, wsHandler)

	// 6. 启动服务
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
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		slog.Error("Fiber shutdown error", "error", err)
	}
}
