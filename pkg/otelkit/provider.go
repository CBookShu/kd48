package otelkit

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer 初始化 OTel TracerProvider，支持 OTLP 导出
// 如果设置了 OTEL_EXPORTER_OTLP_ENDPOINT 环境变量，使用 OTLP HTTP 导出
// 否则使用标准输出导出（本地开发）
func InitTracer(serviceName string) (func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint != "" {
		// 使用 OTLP HTTP 导出（连接 Jaeger）
		exporter, err = otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
	} else {
		// 使用标准输出导出器（本地开发阶段）
		exporter, err = stdouttrace.New()
		if err != nil {
			return nil, err
		}
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

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

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
