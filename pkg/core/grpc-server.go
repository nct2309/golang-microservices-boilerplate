package core

import (
	"context"
	"fmt"
	"golang-microservices-boilerplate/pkg/utils"
	"net"
	"time"

	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// GrpcServerConfig contains configuration for gRPC server
type GrpcServerConfig struct {
	Host                  string
	Port                  string
	MaxConnectionIdle     time.Duration
	MaxConnectionAge      time.Duration
	MaxConnectionAgeGrace time.Duration
	KeepAliveTime         time.Duration
	KeepAliveTimeout      time.Duration
}

// DefaultGrpcServerConfig provides sensible defaults for gRPC server configuration
func DefaultGrpcServerConfig() *GrpcServerConfig {
	return &GrpcServerConfig{
		Host:                  "0.0.0.0",
		Port:                  utils.GetEnv("GRPC_PORT", "9090"),
		MaxConnectionIdle:     15 * time.Minute,
		MaxConnectionAge:      30 * time.Minute,
		MaxConnectionAgeGrace: 5 * time.Second,
		KeepAliveTime:         5 * time.Minute,
		KeepAliveTimeout:      20 * time.Second,
	}
}

// BaseGrpcServer represents the gRPC server for a microservice
type BaseGrpcServer struct {
	server   *grpc.Server
	Config   *GrpcServerConfig
	Logger   Logger
	listener net.Listener
}

// NewBaseGrpcServer creates a new base gRPC server with default config
func NewBaseGrpcServer(logger Logger) *BaseGrpcServer {
	return NewBaseGrpcServerWithConfig(logger, DefaultGrpcServerConfig())
}

// NewBaseGrpcServerWithConfig creates a new base gRPC server with custom config
func NewBaseGrpcServerWithConfig(logger Logger, config *GrpcServerConfig) *BaseGrpcServer {
	// Set up server interceptors
	recoveryHandler := func(p interface{}) (err error) {
		logger.Error("Recovered from panic in gRPC handler", "panic", p)
		return status.Errorf(codes.Internal, "internal server error")
	}

	opts := []grpc_recovery.Option{
		grpc_recovery.WithRecoveryHandler(recoveryHandler),
	}

	// Create gRPC server with middleware
	server := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     config.MaxConnectionIdle,
			MaxConnectionAge:      config.MaxConnectionAge,
			MaxConnectionAgeGrace: config.MaxConnectionAgeGrace,
			Time:                  config.KeepAliveTime,
			Timeout:               config.KeepAliveTimeout,
		}),
		grpc.ChainUnaryInterceptor(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_validator.UnaryServerInterceptor(),
			grpc_recovery.UnaryServerInterceptor(opts...),
		),
		grpc.ChainStreamInterceptor(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_validator.StreamServerInterceptor(),
			grpc_recovery.StreamServerInterceptor(opts...),
		),
	)

	// Enable reflection for debugging
	reflection.Register(server)

	return &BaseGrpcServer{
		server: server,
		Config: config,
		Logger: logger,
	}
}

// Start begins listening for gRPC requests
func (s *BaseGrpcServer) Start() error {
	addr := fmt.Sprintf("%s:%s", s.Config.Host, s.Config.Port)
	s.Logger.Info("Starting gRPC server", "address", addr)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			s.Logger.Error("Failed to serve gRPC", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server
func (s *BaseGrpcServer) Stop() {
	s.Logger.Info("Stopping gRPC server")
	s.server.GracefulStop()
	if s.listener != nil {
		s.listener.Close()
	}
}

// Server returns the underlying gRPC server
func (s *BaseGrpcServer) Server() *grpc.Server {
	return s.server
}

// BaseGrpcHandler defines the interface that all gRPC handlers should implement
type BaseGrpcHandler interface {
	// RegisterGrpcHandlers registers all gRPC service handlers with the server
	RegisterGrpcHandlers(server *grpc.Server)
}

// GrpcClientConfig contains configuration for gRPC client connections
type GrpcClientConfig struct {
	ServiceName            string
	ServiceHost            string
	ServicePort            int
	DialTimeout            time.Duration
	KeepAlive              time.Duration
	KeepAliveTimeout       time.Duration
	AllowInsecureTransport bool
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
		AllowInsecureTransport: false, // Set to true for development
	}
}

// BaseGrpcClient provides a base implementation for gRPC clients
type BaseGrpcClient struct {
	Conn   *grpc.ClientConn
	Config *GrpcClientConfig
	Logger Logger
}

// NewBaseGrpcClient creates a new gRPC client connection
func NewBaseGrpcClient(logger Logger, config *GrpcClientConfig) (*BaseGrpcClient, error) {
	dialOptions := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAlive,
			Timeout:             config.KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
	}

	// Handle transport security
	if config.AllowInsecureTransport {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// In production, you'd use secure credentials here
		// credentials, err := loadTLSCredentials()
		// if err != nil {
		//     return nil, err
		// }
		// dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials))

		// For now, using insecure for simplicity
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Connect to the server
	addr := fmt.Sprintf("%s:%d", config.ServiceHost, config.ServicePort)
	logger.Info("Connecting to gRPC service", "service", config.ServiceName, "address", addr)

	conn, err := grpc.NewClient(
		addr,
		dialOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", config.ServiceName, err)
	}

	return &BaseGrpcClient{
		Conn:   conn,
		Config: config,
		Logger: logger,
	}, nil
}

// Close closes the client connection
func (c *BaseGrpcClient) Close() error {
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

// WithContext returns a new context with timeout for gRPC calls
func (c *BaseGrpcClient) WithContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parentCtx, c.Config.DialTimeout)
}
