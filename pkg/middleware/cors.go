package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// CORSMiddleware handles Cross-Origin Resource Sharing (CORS) requests
func CORSMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// how to add above cors to allow https
// add the following to the middleware
// c.Set("Access-Control-Allow-Origin", "*")
// c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
// c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
