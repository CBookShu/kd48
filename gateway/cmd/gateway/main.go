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

	// 3. 模拟一个请求处理的完整链路
	slog.Info("Gateway starting...")

	// 假设这是一个 WS 连接建立后的上下文
	ctx, span := tracer.Start(context.Background(), "MockWSConnection")
	defer span.End()

	// 【核心动作】：将 OTel 的 TraceID 注入到 context 中
	ctx = otelkit.InjectTraceIDToCtx(ctx)

	// 4. 打印日志 (此时传入了携带 trace_id 的 ctx)
	slog.InfoContext(ctx, "New client connected via WS", "client_ip", "192.168.1.100")

	// 模拟内部调用（虽然现在没写 gRPC，但日志已经串起来了）
	time.Sleep(100 * time.Millisecond)
	slog.InfoContext(ctx, "User login request received", "user_id", 123)

	// 阻塞等待退出信号 (Ctrl+C)
	slog.Info("Server is running, press Ctrl+C to stop...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")
}
