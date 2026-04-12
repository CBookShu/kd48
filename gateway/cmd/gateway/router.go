package main

import (
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/gateway/internal/ws"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace"
)

// SetupRoutes 显式地将路由规则挂载到 Fiber App 上
func SetupRoutes(app *fiber.App, userClient userv1.UserServiceClient, tracer trace.Tracer) {
	// 健康检查
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// WebSocket 路由
	// 必须先经过 IsWebSocketUpgrade 中间件拦截非 WS 请求
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	wsHandler := ws.NewHandler(userClient, tracer)
	// websocket.New 会自动完成握手，并将 conn 存入 c.Locals("websocket")
	app.Get("/ws", websocket.New(wsHandler.ServeWS))
}
