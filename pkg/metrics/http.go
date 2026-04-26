package metrics

import (
	"strconv"
	"time"
	
	"github.com/gofiber/fiber/v2"
)

func FiberMiddleware(serviceName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		path := c.Route().Path
		method := c.Method()
		
		HTTPRequestsTotal.WithLabelValues(serviceName, method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(serviceName, method, path).Observe(duration)
		
		return err
	}
}