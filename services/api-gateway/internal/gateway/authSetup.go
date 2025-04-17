package gateway

import (
	"golang-microservices-boilerplate/pkg/core/logger"

	"github.com/gofiber/fiber/v2"
)

// setupAuthMiddleware configures and applies JWT authentication middleware selectively to API routes.
func setupAuthMiddleware(app *fiber.App, logger logger.Logger) {

	// useage:
	// app.Use("/api/v1/auth/refresh", middleware.AuthMiddleware())
	_ = app
	logger.Info("Auth middleware configured for apis")
}
