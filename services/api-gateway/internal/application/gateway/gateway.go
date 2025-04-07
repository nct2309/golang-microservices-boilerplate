package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"

	"golang-microservices-boilerplate/pkg/core"
	"golang-microservices-boilerplate/pkg/middleware"
	user_pb "golang-microservices-boilerplate/proto/user-service"
	"golang-microservices-boilerplate/services/api-gateway/internal/domain"
)

// Gateway handles HTTP requests by translating them to gRPC calls using Fiber
type Gateway struct {
	ctx               context.Context
	app               *fiber.App
	gwMux             *runtime.ServeMux
	logger            core.Logger
	stdLogger         *log.Logger // Keep standard logger for compatibility
	fiberLoggerConfig *fiberLogger.Config
	discovery         domain.ServiceDiscovery
	serviceConns      map[string]*grpc.ClientConn
	opts              []grpc.DialOption
	mu                sync.Mutex
}

// GatewayOption configures the Gateway
type GatewayOption func(*Gateway)

// WithLogger sets the structured logger for the gateway
func WithLogger(logger core.Logger) GatewayOption {
	return func(g *Gateway) {
		g.logger = logger

		// Create a standard logger adapter for compatibility
		stdLogWriter := &stdLogAdapter{logger: logger}
		g.stdLogger = log.New(stdLogWriter, "", 0)
	}
}

// WithStdLogger sets a standard logger for the gateway (for backward compatibility)
func WithStdLogger(logger *log.Logger) GatewayOption {
	return func(g *Gateway) {
		g.stdLogger = logger
	}
}

// stdLogAdapter adapts core.Logger to io.Writer for standard logger
type stdLogAdapter struct {
	logger core.Logger
}

// Write implements io.Writer for the standard logger adapter
func (a *stdLogAdapter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if strings.HasPrefix(msg, "ERROR") || strings.Contains(msg, "error") {
		a.logger.Error(msg)
	} else {
		a.logger.Info(msg)
	}
	return len(p), nil
}

// NewGateway creates a new Gateway using Fiber
func NewGateway(
	ctx context.Context,
	discovery domain.ServiceDiscovery,
	opts ...GatewayOption,
) *Gateway {
	// Create a default structured logger
	defaultLogger, err := core.NewLoggerFromEnv()
	if err != nil {
		defaultLogger, _ = core.NewLogger(core.DefaultLogConfig())
	}

	// Configure gRPC logger
	grpcLoggerWriter := &stdLogAdapter{logger: defaultLogger.Named("grpc")}
	grpcStdLogger := log.New(grpcLoggerWriter, "", 0)
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(grpcStdLogger.Writer(), grpcStdLogger.Writer(), grpcStdLogger.Writer()))

	// Configure Fiber logger
	fiberLogConfig := fiberLogger.ConfigDefault
	fiberLogConfig.Format = "[${time}] [${ip}:${port}] ${status} - ${method} ${path}\n"

	// Define the error handler function
	customErrorHandler := func(c *fiber.Ctx, err error) error {
		// We don't have access to 'g' here directly, so we log with the default logger
		defaultLogger.Error("Fiber Error", "error", err, "path", c.Path(), "method", c.Method())
		return fiber.DefaultErrorHandler(c, err)
	}

	g := &Gateway{
		ctx: ctx,
		app: fiber.New(fiber.Config{
			ErrorHandler: customErrorHandler,
		}),
		gwMux: runtime.NewServeMux(
			runtime.WithErrorHandler(errorHandler),
			runtime.WithIncomingHeaderMatcher(headerMatcher),
		),
		discovery:         discovery,
		serviceConns:      make(map[string]*grpc.ClientConn),
		opts:              []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		logger:            defaultLogger.Named("gateway"),
		stdLogger:         log.New(os.Stdout, "[API-GATEWAY] ", log.LstdFlags),
		fiberLoggerConfig: &fiberLogConfig,
		mu:                sync.Mutex{},
	}

	// Apply options
	for _, opt := range opts {
		opt(g)
	}

	// Add Fiber middleware
	g.app.Use(cors.New())
	g.app.Use(middleware.LoggerMiddleware()) // Use our custom logger middleware

	// Mount the gRPC-Gateway mux
	g.app.Use("/api", adaptor.HTTPHandler(g.gwMux))

	return g
}

// Start initializes the gateway and starts the Fiber HTTP server
func (g *Gateway) Start(port string) error {
	// Setup connections and register handlers with the gRPC-Gateway mux (g.gwMux)
	if err := g.setupHandlers(); err != nil {
		return err
	}

	// Register Swagger handler directly with the Fiber app (g.app)
	swaggerDir := os.Getenv("SWAGGER_DIR")
	if swaggerDir == "" {
		// Try to find swagger directory in different possible locations
		possiblePaths := []string{
			"services/api-gateway/swagger", // Development path
			"swagger",                      // Docker container path
			"./swagger",                    // Relative path
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				g.logger.Info("Found swagger directory", "path", path)
				swaggerDir = path

				// Check if proto subdir exists
				protoDir := fmt.Sprintf("%s/proto", path)
				if _, err := os.Stat(protoDir); err == nil {
					g.logger.Info("Found proto directory", "path", protoDir)
				} else {
					g.logger.Info("Proto directory not found", "path", protoDir, "error", err)
				}

				break
			}
		}
	}

	if swaggerDir == "" {
		g.logger.Warn("Swagger directory not found in any of the expected locations")
	} else {
		g.RegisterSwaggerUI(swaggerDir)
	}

	// Register health check directly with the Fiber app (g.app)
	g.app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "healthy"})
	})

	// Start Fiber HTTP server
	g.logger.Info("Starting Fiber HTTP server", "port", port)
	return g.app.Listen(fmt.Sprintf(":%s", port))
}

// Shutdown gracefully shuts down the Fiber server and gRPC connections
func (g *Gateway) Shutdown(ctx context.Context) error {
	g.logger.Info("Shutting down Fiber server...")
	serverErr := g.app.Shutdown()

	g.logger.Info("Closing gRPC connections...")
	g.mu.Lock()
	defer g.mu.Unlock()

	var closeErrors []string
	for service, conn := range g.serviceConns {
		if err := conn.Close(); err != nil {
			errMsg := fmt.Sprintf("Error closing connection to %s: %v", service, err)
			g.logger.Error("Failed to close connection", "service", service, "error", err)
			closeErrors = append(closeErrors, errMsg)
		}
	}

	if serverErr != nil {
		g.logger.Error("Failed to shutdown Fiber server", "error", serverErr)
		closeErrors = append(closeErrors, fmt.Sprintf("Fiber server shutdown error: %v", serverErr))
	}

	if len(closeErrors) > 0 {
		return fmt.Errorf("errors during shutdown: %s", strings.Join(closeErrors, "; "))
	}

	g.logger.Info("Gateway shutdown complete")
	return nil
}

// setupHandlers registers gRPC-Gateway handlers for all services
func (g *Gateway) setupHandlers() error {
	// Get all services from discovery
	services, err := g.discovery.GetAllServices()
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}

	// Setup handlers for each service
	for _, service := range services {
		switch strings.ToLower(service.Name) {
		case "user", "user-service":
			if err := g.setupUserServiceHandlers(service); err != nil {
				return err
			}
		// Add cases for other services here as you implement them
		default:
			g.logger.Error("Unknown service: %s, skipping handler setup", service.Name)
		}
	}

	return nil
}

// setupUserServiceHandlers registers handlers for the user service with the gRPC-Gateway mux
func (g *Gateway) setupUserServiceHandlers(service domain.Service) error {
	// Register user service handlers directly using the endpoint
	// The gateway mux will create its own internal connection.
	err := user_pb.RegisterUserServiceHandlerFromEndpoint(g.ctx, g.gwMux, service.Endpoint, g.opts)
	if err != nil {
		g.logger.Error("Failed to register user service handler from endpoint", "endpoint", service.Endpoint, "error", err)
		return fmt.Errorf("failed to register user service handler from endpoint %s: %w", service.Endpoint, err)
	}

	// We might still want to store the original connection from discovery for other purposes
	// or for cleanup, but it's not directly used by the Register...FromEndpoint call.
	// Let's get it and store it for potential shutdown cleanup, but acknowledge it's separate.
	g.mu.Lock()
	conn, connErr := g.discovery.GetConnection(service.Name) // Attempt to get connection for potential cleanup
	if connErr == nil {
		if _, exists := g.serviceConns[service.Name]; !exists {
			g.serviceConns[service.Name] = conn // Store original discovery connection if needed for shutdown
		}
	} else {
		// Log if we couldn't get the original connection, but proceed since registration succeeded
		g.logger.Warn("Could not get discovery connection for potential cleanup", "service", service.Name, "error", connErr)
	}
	g.mu.Unlock()

	g.logger.Info("Registered gRPC-Gateway handlers via endpoint",
		"service", "user-service",
		"endpoint", service.Endpoint)
	return nil
}

// errorHandler is the gRPC-Gateway error handler
func errorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	// Since this is a standalone function without access to the gateway instance,
	// we use the standard log package. In a production environment, you might want
	// to inject a logger here.
	log.Printf("[gRPC-Gateway Error]: %v", err)

	runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
}

// headerMatcher is the gRPC-Gateway header matcher
func headerMatcher(key string) (string, bool) {
	key = strings.ToLower(key)
	if key == "authorization" {
		return key, true
	}

	if strings.HasPrefix(key, "x-") {
		return key, true
	}

	return runtime.DefaultHeaderMatcher(key)
}
