package main

import (
	"context"
	"log"
	"log/slog"

	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
)

func main() {
	// 默认从当前工作目录(项目根目录)读取
	c, err := conf.Load("./config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	handler := logzap.New(c.Log.Level)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	slog.InfoContext(ctx, "Gateway starting up...", "module", "gateway", "port", c.Gateway.Port)
	slog.WarnContext(ctx, "This is a warning message", "reason", "test zap bridge")
	slog.DebugContext(ctx, "Debugging data", "map_data", map[string]int{"a": 1, "b": 2})
}
