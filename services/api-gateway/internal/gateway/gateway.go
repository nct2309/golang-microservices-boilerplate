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
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"

	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/middleware"
	"golang-microservices-boilerplate/services/api-gateway/internal/domain"
)

// Gateway handles HTTP requests by translating them to gRPC calls using Fiber
type Gateway struct {
	ctx          context.Context
	app          *fiber.App
	gwMux        *runtime.ServeMux
	logger       logger.Logger
	stdLogger    *log.Logger // Standard logger adapter for compatibility
	discovery    domain.ServiceDiscovery
	serviceConns map[string]*grpc.ClientConn
	opts         []grpc.DialOption
	mu           sync.Mutex
}

// GatewayOption configures the Gateway
type GatewayOption func(*Gateway)

// WithLogger sets the structured logger for the gateway
func WithLogger(l logger.Logger) GatewayOption {
	return func(g *Gateway) {
		g.logger = l
		stdLogWriter := &stdLogAdapter{logger: l}
		g.stdLogger = log.New(stdLogWriter, "", 0)
	}
}

// stdLogAdapter adapts logger.Logger to io.Writer for standard logger
type stdLogAdapter struct {
	logger logger.Logger
}

// Write implements io.Writer for the standard logger adapter
func (a *stdLogAdapter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	if strings.HasPrefix(msg, "ERROR") || strings.Contains(msg, "error") {
		a.logger.Error(msg)
	} else if strings.HasPrefix(msg, "WARN") || strings.Contains(msg, "warning") {
		a.logger.Warn(msg)
	} else if strings.HasPrefix(msg, "DEBUG") {
		a.logger.Debug(msg)
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

	// Create a temporary base logger first
	tempBaseLogger, _ := logger.NewLogger(logger.DefaultLogConfig())
	// Now create a named logger from the base instance
	tempLogger := tempBaseLogger.Named("gateway-init")

	g := &Gateway{
		ctx: ctx,
		// Fiber app initialized later after logger is finalized
		gwMux: runtime.NewServeMux(
			runtime.WithErrorHandler(defaultErrorHandler),
			runtime.WithIncomingHeaderMatcher(headerMatcher),
		),
		discovery:    discovery,
		serviceConns: make(map[string]*grpc.ClientConn),
		opts:         []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		logger:       tempLogger, // Start with temp named logger
		stdLogger:    log.New(&stdLogAdapter{logger: tempLogger}, "", 0),
		mu:           sync.Mutex{},
	}

	// Apply options, potentially overriding the logger
	for _, opt := range opts {
		opt(g)
	}

	// --- Configure components that depend on the FINAL logger ---

	// Configure Fiber App with the final logger in the error handler
	g.app = fiber.New(fiber.Config{
		ErrorHandler: g.fiberErrorHandler, // Assign the method reference
	})

	// Configure gRPC global logger
	grpcLoggerWriter := &stdLogAdapter{logger: g.logger.Named("grpc")}
	grpcStdLogger := log.New(grpcLoggerWriter, "", 0)
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(grpcStdLogger.Writer(), grpcStdLogger.Writer(), grpcStdLogger.Writer()))

	// Add Fiber middleware
	g.app.Use(cors.New())                    // CORS
	g.app.Use(middleware.LoggerMiddleware()) // Call middleware without logger arg

	setupAuthMiddleware(g.app, g.logger)

	// Mount the gRPC-Gateway mux
	g.app.Use("/api", adaptor.HTTPHandler(g.gwMux))

	return g
}

// fiberErrorHandler is the custom error handler for Fiber that uses the gateway's logger.
func (g *Gateway) fiberErrorHandler(c *fiber.Ctx, err error) error {
	g.logger.Error("Fiber Error", "error", err, "path", c.Path(), "method", c.Method(), "ip", c.IP())
	return fiber.DefaultErrorHandler(c, err)
}

// Start initializes the gateway and starts the Fiber HTTP server
func (g *Gateway) Start(port string) error {
	if err := g.setupHandlers(); err != nil {
		return err
	}

	swaggerDir := os.Getenv("SWAGGER_DIR")
	if swaggerDir == "" {
		possiblePaths := []string{
			"swagger",
			"./swagger",
		}
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				g.logger.Info("Found swagger directory", "path", path)
				swaggerDir = path
				break
			}
		}
	}

	if swaggerDir == "" {
		g.logger.Warn("Swagger directory not found, skipping Swagger UI setup")
	} else {
		g.RegisterSwaggerUI(swaggerDir)
	}

	g.app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "healthy"})
	})

	g.logger.Info("Starting Fiber HTTP server", "port", port)
	return g.app.Listen(fmt.Sprintf(":%s", port))
}

// Shutdown gracefully shuts down the Fiber server
func (g *Gateway) Shutdown(ctx context.Context) error {
	g.logger.Info("Shutting down Fiber server...")
	serverErr := g.app.Shutdown()

	// Removed closing of gRPC connections previously managed by discovery
	// The connections used by Register...FromEndpoint are managed internally by grpc-gateway/grpc

	if serverErr != nil {
		g.logger.Error("Failed to shutdown Fiber server", "error", serverErr)
		return fmt.Errorf("fiber server shutdown error: %w", serverErr)
	}

	g.logger.Info("Gateway shutdown complete")
	return nil
}

// defaultErrorHandler is the default gRPC-Gateway error handler.
func defaultErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	grpclog.Errorf("gRPC-Gateway Error: %v", err)
	runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
}

// headerMatcher remains the same.
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
