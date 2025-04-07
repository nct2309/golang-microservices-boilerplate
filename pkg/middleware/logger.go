package middleware

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Logger is a middleware function that logs incoming requests and responses
func LoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process the request
		err := c.Next()

		// Log the request and response details
		log.Printf("Request: %s %s | Response Status: %d | Duration: %s",
			c.Method(), c.Path(), c.Response().StatusCode(), time.Since(start))

		return err
	}
}
