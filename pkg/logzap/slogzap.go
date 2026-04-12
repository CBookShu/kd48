package logzap

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ZapHandler struct {
	level  slog.Leveler
	logger *zap.Logger
}

func New(level string, filePath string) *ZapHandler {
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

	// 将 slog.Level 转换为 zap.Level，用于控制底层 Core
	var zapLevel zapcore.Level
	switch l {
	case slog.LevelDebug:
		zapLevel = zapcore.DebugLevel
	case slog.LevelInfo:
		zapLevel = zapcore.InfoLevel
	case slog.LevelWarn:
		zapLevel = zapcore.WarnLevel
	case slog.LevelError:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// 🚨 修改点：精细化控制输出格式
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder   // 人类可读时间：2024-01-01T15:04:05.000Z
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder // 短路径 Caller：slogzap/logzap.go:25
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // 大写 INFO/WARN

	jsonEncoder := zapcore.NewJSONEncoder(encoderConfig)

	consoleEncoderConfig := encoderConfig
	// 终端日志加上颜色区分级别 (比如 ERROR 是红色，INFO 是蓝色)
	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

	// 🚨 修改点：构建多目标 Core (Tee)
	var cores []zapcore.Core

	// 目标 1：标准输出 (保持原样)
	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapLevel))

	// 目标 2：文件输出
	if filePath != "" {
		dir := filepath.Dir(filePath)
		// 启动时自动创建日志目录
		if err := os.MkdirAll(dir, 0755); err != nil {
			// 此时 slog 可能还没完全就绪，直接写 stderr
			os.Stderr.WriteString("Failed to create log dir: " + err.Error() + "\n")
		} else {
			// 引入 lumberjack 实现滚动切割
			hook := &lumberjack.Logger{
				Filename:   filePath,
				MaxSize:    100,  // MB
				MaxBackups: 3,    // 保留旧文件最大个数
				MaxAge:     30,   // 保留最大天数
				Compress:   true, // 是否压缩
			}
			cores = append(cores, zapcore.NewCore(jsonEncoder, zapcore.AddSync(hook), zapLevel))
		}
	}

	// 合并 Core
	core := zapcore.NewTee(cores...)

	// 构造底层 Logger (保留原有的 CallerSkip 以对齐业务代码行号)
	baseLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(3))

	return &ZapHandler{
		level:  l,
		logger: baseLogger,
	}
}

func (h *ZapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *ZapHandler) Handle(ctx context.Context, r slog.Record) error {
	traceId := "N/A"
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		traceId = spanCtx.TraceID().String()
	}

	fields := []zapcore.Field{
		zap.String("trace_id", traceId),
	}

	r.Attrs(func(attr slog.Attr) bool {
		fields = appendAttrToFields(fields, attr)
		return true
	})

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

func appendAttrToFields(fields []zapcore.Field, attr slog.Attr) []zapcore.Field {
	if attr.Equal(slog.Attr{}) {
		return fields
	}
	if attr.Value.Kind() == slog.KindGroup {
		return append(fields, zap.Any(attr.Key, attr.Value.Group()))
	}

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
