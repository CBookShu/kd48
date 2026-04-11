package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/rediskit"
	"github.com/CBookShu/kd48/pkg/registry"
	"go.opentelemetry.io/otel"
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

	// 3. 初始化 Redis
	rdb, err := rediskit.NewClient(c.Redis)
	if err != nil {
		slog.Error("Redis init failed", "error", err)
		panic(err)
	}
	defer rdb.Close()
	slog.Info("Redis connected", "addr", c.Redis.Addr)

	// 4. 初始化 Etcd
	etcd, err := registry.NewClient(c.Etcd)
	if err != nil {
		slog.Error("Etcd init failed", "error", err)
		panic(err)
	}
	defer etcd.Close()
	slog.Info("Etcd connected", "endpoints", c.Etcd.Endpoints)

	// 5. 模拟一个带完整追踪的请求链路
	ctx, span := tracer.Start(context.Background(), "MockWSConnection")
	defer span.End()
	ctx = otelkit.InjectTraceIDToCtx(ctx)

	slog.InfoContext(ctx, "Gateway fully booted, all dependencies ready",
		"redis", c.Redis.Addr,
		"etcd", c.Etcd.Endpoints,
		"port", c.Gateway.Port,
	)

	// 模拟写入 Redis
	err = rdb.Set(ctx, "test:key", "hello_kd48", 10*time.Second).Err()
	if err != nil {
		slog.ErrorContext(ctx, "Redis set failed", "error", err)
	} else {
		slog.InfoContext(ctx, "Redis write succeeded")
	}

	// 阻塞等待退出信号
	slog.Info("Server is running, press Ctrl+C to stop...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")
}
