package grpc

import (
	"fmt"
	"time"

	"golang-microservices-boilerplate/pkg/core/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// GrpcClientConfig contains configuration for gRPC client connections
type GrpcClientConfig struct {
	ServiceName            string
	ServiceHost            string
	ServicePort            int
	DialTimeout            time.Duration
	KeepAlive              time.Duration
	KeepAliveTimeout       time.Duration
	AllowInsecureTransport bool // Should be false in production
}

// DefaultGrpcClientConfig provides sensible defaults for gRPC client configuration
func DefaultGrpcClientConfig(serviceName string, serviceHost string, servicePort int) *GrpcClientConfig {
	return &GrpcClientConfig{
		ServiceName:            serviceName,
		ServiceHost:            serviceHost,
		ServicePort:            servicePort,
		DialTimeout:            5 * time.Second,
		KeepAlive:              30 * time.Second,
		KeepAliveTimeout:       10 * time.Second,
		AllowInsecureTransport: true, // Defaulting to true for easier local dev/testing
	}
}

// BaseGrpcClient provides a base implementation for gRPC clients
type BaseGrpcClient struct {
	Conn   *grpc.ClientConn
	Config *GrpcClientConfig
	Logger logger.Logger
}

// NewBaseGrpcClient creates a new gRPC client connection
func NewBaseGrpcClient(logger logger.Logger, config *GrpcClientConfig) (*BaseGrpcClient, error) {
	dialOptions := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAlive,
			Timeout:             config.KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
		// Add interceptors if needed (e.g., logging, tracing)
		// grpc.WithUnaryInterceptor(...),
	}

	// Handle transport security
	if config.AllowInsecureTransport {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// TODO: Implement secure credentials loading for production
		// For example, using TLS:
		// creds, err := credentials.NewClientTLSFromFile("path/to/ca.crt", "service.domain.com")
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		// }
		// dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
		return nil, fmt.Errorf("secure transport requested but not implemented (AllowInsecureTransport=false)")
	}

	// Connect to the server
	addr := fmt.Sprintf("%s:%d", config.ServiceHost, config.ServicePort)
	logger.Info("Connecting to gRPC service", "service", config.ServiceName, "address", addr)

	// Dial the server (Note: Context with timeout is often used with grpc.WithBlock())
	// ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	// defer cancel()

	conn, err := grpc.NewClient(addr, dialOptions...)
	if err != nil {
		logger.Error("Failed to connect to gRPC service", "service", config.ServiceName, "address", addr, "error", err)
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", config.ServiceName, addr, err)
	}

	logger.Info("Successfully connected to gRPC service", "service", config.ServiceName, "address", addr)

	return &BaseGrpcClient{
		Conn:   conn,
		Config: config,
		Logger: logger,
	}, nil
}

// Close closes the client connection
func (c *BaseGrpcClient) Close() error {
	if c.Conn != nil {
		c.Logger.Info("Closing gRPC client connection", "service", c.Config.ServiceName)
		return c.Conn.Close()
	}
	return nil
}

// WithContext returns a new context with timeout suitable for gRPC calls using the client's DialTimeout.
// DEPRECATED: Prefer creating context with timeout directly where the call is made.
// func (c *BaseGrpcClient) WithContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
// 	 return context.WithTimeout(parentCtx, c.Config.DialTimeout)
// }
