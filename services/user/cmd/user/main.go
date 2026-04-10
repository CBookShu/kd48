package main

import (
	"context"
	"log/slog"

	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
)

func main() {
	c, err := conf.Load("./config.yaml")
	if err != nil {
		panic(err)
	}

	handler := logzap.New(c.Log.Level)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	slog.InfoContext(ctx, "User service starting up...", "module", "user_service", "port", c.UserService.Port)
}
