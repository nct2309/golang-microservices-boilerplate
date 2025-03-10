package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

// Server defines the basic interface for all servers
type Server interface {
	Start() error
	Stop() error
	RegisterRoutes(registerFunc func(app *fiber.App))
}

// ServerConfig contains all configuration parameters for the server
type ServerConfig struct {
	Host           string
	Port           int
	ShutdownDelay  time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	AppName        string
	AllowOrigins   string
	AllowMethods   string
	AllowHeaders   string
	LoggingEnabled bool
}

// DefaultServerConfig returns a default configuration for the server
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:           "0.0.0.0",
		Port:           8080,
		ShutdownDelay:  5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		AppName:        "Fiber Microservice",
		AllowOrigins:   "*",
		AllowMethods:   "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:   "Origin, Content-Type, Accept, Authorization",
		LoggingEnabled: true,
	}
}

// LoadConfigFromEnv loads server configuration from environment variables
func LoadConfigFromEnv() *ServerConfig {
	// Load .env file if it exists
	godotenv.Load()

	config := DefaultServerConfig()

	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Host = host
	}

	if portStr := os.Getenv("SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	if shutdownStr := os.Getenv("SERVER_SHUTDOWN_DELAY"); shutdownStr != "" {
		if delay, err := time.ParseDuration(shutdownStr); err == nil {
			config.ShutdownDelay = delay
		}
	}

	if readTimeoutStr := os.Getenv("SERVER_READ_TIMEOUT"); readTimeoutStr != "" {
		if timeout, err := time.ParseDuration(readTimeoutStr); err == nil {
			config.ReadTimeout = timeout
		}
	}

	if writeTimeoutStr := os.Getenv("SERVER_WRITE_TIMEOUT"); writeTimeoutStr != "" {
		if timeout, err := time.ParseDuration(writeTimeoutStr); err == nil {
			config.WriteTimeout = timeout
		}
	}

	if appName := os.Getenv("SERVER_APP_NAME"); appName != "" {
		config.AppName = appName
	}

	if origins := os.Getenv("SERVER_ALLOW_ORIGINS"); origins != "" {
		config.AllowOrigins = origins
	}

	if methods := os.Getenv("SERVER_ALLOW_METHODS"); methods != "" {
		config.AllowMethods = methods
	}

	if headers := os.Getenv("SERVER_ALLOW_HEADERS"); headers != "" {
		config.AllowHeaders = headers
	}

	if logEnabledStr := os.Getenv("SERVER_LOGGING_ENABLED"); logEnabledStr != "" {
		config.LoggingEnabled = logEnabledStr == "true" || logEnabledStr == "1"
	}

	return config
}

// FiberServer implements the Server interface using Fiber
type FiberServer struct {
	Config         *ServerConfig
	app            *fiber.App
	routeRegistrar func(app *fiber.App)
	Logger         Logger
}

// NewFiberServer creates a new instance of FiberServer with environment-based configuration
func NewFiberServer(logger Logger) *FiberServer {
	return &FiberServer{
		Config: LoadConfigFromEnv(),
		Logger: logger,
	}
}

// NewFiberServerWithConfig creates a new instance with custom configuration
func NewFiberServerWithConfig(config *ServerConfig, logger Logger) *FiberServer {
	return &FiberServer{
		Config: config,
		Logger: logger,
	}
}

// RegisterRoutes accepts a function that registers routes to the Fiber app
func (s *FiberServer) RegisterRoutes(registerFunc func(app *fiber.App)) {
	s.routeRegistrar = registerFunc
}

// setupMiddleware configures default middleware for the server
func (s *FiberServer) setupMiddleware() {
	s.app.Use(recover.New())

	if s.Config.LoggingEnabled {
		s.app.Use(logger.New())
	}

	s.app.Use(cors.New(cors.Config{
		AllowOrigins:     s.Config.AllowOrigins,
		AllowMethods:     s.Config.AllowMethods,
		AllowHeaders:     s.Config.AllowHeaders,
		AllowCredentials: true,
	}))
}

// Start initializes and starts the Fiber server
func (s *FiberServer) Start() error {
	// Initialize Fiber app with configurations
	s.app = fiber.New(fiber.Config{
		ReadTimeout:  s.Config.ReadTimeout,
		WriteTimeout: s.Config.WriteTimeout,
		AppName:      s.Config.AppName,
	})

	// Setup middleware
	s.setupMiddleware()

	// Register routes if provided
	if s.routeRegistrar != nil {
		s.routeRegistrar(s.app)
	}

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		s.Logger.Info("Server is shutting down...")
		_ = s.Stop()
	}()

	// Start server
	serverAddr := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)
	s.Logger.Info("Server starting", "address", serverAddr)

	return s.app.Listen(serverAddr)
}

// Stop gracefully shuts down the server
func (s *FiberServer) Stop() error {
	if s.app == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Config.ShutdownDelay)
	defer cancel()

	return s.app.ShutdownWithContext(ctx)
}
