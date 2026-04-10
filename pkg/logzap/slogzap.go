package logzap

import (
	"context"
	"log/slog"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapHandler struct {
	level  slog.Leveler
	logger *zap.Logger
}

func New(level string) *ZapHandler {
	var l slog.Leveler
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	// 使用 Zap 新版标准配置方式
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"
	cfg.EncoderConfig.TimeKey = "ts"

	core, err := cfg.Build(zap.AddCallerSkip(2))
	if err != nil {
		panic(err)
	}

	return &ZapHandler{
		level:  l,
		logger: core,
	}
}

func (h *ZapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *ZapHandler) Handle(ctx context.Context, r slog.Record) error {
	// 提取 TraceID (预留 OTel 接口)
	traceId := "N/A"
	if tid, ok := ctx.Value("trace_id").(string); ok && tid != "" {
		traceId = tid
	}

	fields := []zapcore.Field{
		zap.String("trace_id", traceId),
	}

	// 提取 slog 的附加属性
	r.Attrs(func(attr slog.Attr) bool {
		fields = appendAttrToFields(fields, attr)
		return true
	})

	// 映射 slog.Level 到 zap.Level
	var zapLevel zapcore.Level
	switch {
	case r.Level >= slog.LevelError:
		zapLevel = zapcore.ErrorLevel
	case r.Level >= slog.LevelWarn:
		zapLevel = zapcore.WarnLevel
	case r.Level >= slog.LevelInfo:
		zapLevel = zapcore.InfoLevel
	default:
		zapLevel = zapcore.DebugLevel
	}

	// 使用 zap 的高层 Check/Write 机制
	if ce := h.logger.Check(zapLevel, r.Message); ce != nil {
		ce.Write(fields...)
	}
	return nil
}

func (h *ZapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	var fields []zapcore.Field
	for _, a := range attrs {
		fields = appendAttrToFields(fields, a)
	}
	clone.logger = h.logger.With(fields...)
	return &clone
}

func (h *ZapHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.logger = h.logger.With(zap.Namespace(name))
	return &clone
}

// appendAttrToFields 将 slog.Attr 转换为 zap.Field
func appendAttrToFields(fields []zapcore.Field, attr slog.Attr) []zapcore.Field {
	if attr.Equal(slog.Attr{}) {
		return fields
	}

	// 处理 Group (为了保持 JSON 嵌套结构，直接走 zap.Any 反射，最安全稳定)
	if attr.Value.Kind() == slog.KindGroup {
		return append(fields, zap.Any(attr.Key, attr.Value.Group()))
	}

	// 常规类型强转（零反射开销）
	switch attr.Value.Kind() {
	case slog.KindString:
		return append(fields, zap.String(attr.Key, attr.Value.String()))
	case slog.KindInt64:
		return append(fields, zap.Int64(attr.Key, attr.Value.Int64()))
	case slog.KindFloat64:
		return append(fields, zap.Float64(attr.Key, attr.Value.Float64()))
	case slog.KindBool:
		return append(fields, zap.Bool(attr.Key, attr.Value.Bool()))
	default:
		return append(fields, zap.Any(attr.Key, attr.Value))
	}
}
