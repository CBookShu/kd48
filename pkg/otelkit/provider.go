package otelkit

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer 初始化 OTel TracerProvider
// 目前使用标准输出导出（方便本地调试），后续替换为 Jaeger/OTLP 导出器即可
func InitTracer(serviceName string) (func(context.Context) error, error) {
	// 使用标准输出导出器 (本地开发阶段最直观)
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(), // 美化输出，上线前去掉
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	// 注册为全局 Provider
	otel.SetTracerProvider(tp)

	// 返回优雅关闭函数
	return tp.Shutdown, nil
}

// InjectTraceIDToCtx 从 OTel Context 中提取 TraceID，并注入到 context 中
// 这样 slogzap 就可以通过 ctx.Value("trace_id") 拿到了，实现了日志与 OTel 的解耦
func InjectTraceIDToCtx(ctx context.Context) context.Context {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()
	if !spanCtx.IsValid() {
		return ctx
	}
	// 将 16 字节的 TraceID 转换为 32 位十六进制字符串
	traceID := spanCtx.TraceID().String()
	return context.WithValue(ctx, "trace_id", traceID)
}
