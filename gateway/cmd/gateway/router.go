package main

import (
	"github.com/CBookShu/kd48/gateway/internal/ws"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes 显式地将路由规则挂载到 Fiber App 上
// 🚨 修改点：去掉了对具体 userClient 和 tracer 的依赖，只接收组装好的 WsHandler
func SetupRoutes(app *fiber.App, wsHandler *ws.Handler) {
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

	// websocket.New 会自动完成握手，并将 conn 存入 c.Locals("websocket")
	app.Get("/ws", websocket.New(wsHandler.ServeWS))
}
