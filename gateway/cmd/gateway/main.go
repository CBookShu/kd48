package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/rediskit"
	"github.com/CBookShu/kd48/pkg/registry"
	"go.etcd.io/etcd/client/v3/naming/resolver"
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

	tracer := otel.Tracer("github.com/CBookShu/kd48/gateway")

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

	// 1. 将 Etcd 注册为 gRPC 的 Resolver
	etcdResolverBuilder, err := resolver.NewBuilder(etcdCli)
	if err != nil {
		panic(err)
	}
	grpcresolver.Register(etcdResolverBuilder)

	// 2. 使用 Etcd 解析器建立 gRPC 连接
	// 注意格式：etcd:///服务名
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

	// 3. 模拟一个外部 WS 请求触发的内部 gRPC 调用
	ctx, span := tracer.Start(context.Background(), "GatewayMockWSLogin")
	defer span.End()
	ctx = otelkit.InjectTraceIDToCtx(ctx)

	slog.InfoContext(ctx, "Mocking external login request...")

	// 发起 gRPC 调用
	resp, err := userClient.Login(ctx, &userv1.LoginRequest{
		Username: "admin",
		Password: "123456",
	})
	if err != nil {
		slog.ErrorContext(ctx, "Login gRPC call failed", "error", err)
	} else {
		slog.InfoContext(ctx, "Login gRPC call success", "token", resp.Token)
	}

	// 阻塞等待退出信号
	slog.Info("Server is running, press Ctrl+C to stop...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")
}
