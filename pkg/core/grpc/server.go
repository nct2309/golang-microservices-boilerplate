package grpc

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/utils"

	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"google.golang.org/grpc"
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
	Logger   logger.Logger
	listener net.Listener
}

// NewBaseGrpcServer creates a new base gRPC server with default config
func NewBaseGrpcServer(logger logger.Logger) *BaseGrpcServer {
	return NewBaseGrpcServerWithConfig(logger, DefaultGrpcServerConfig())
}

// NewBaseGrpcServerWithConfig creates a new base gRPC server with custom config
func NewBaseGrpcServerWithConfig(logger logger.Logger, config *GrpcServerConfig) *BaseGrpcServer {
	// Set up server interceptors
	recoveryHandler := func(p interface{}) (err error) {
		logger.Error("Recovered from panic in gRPC handler", "panic", p)
		return status.Errorf(http.StatusInternalServerError, "internal server error: %v", p)
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
			grpc_validator.UnaryServerInterceptor(), // Make sure request types have `Validate() error` method
			grpc_recovery.UnaryServerInterceptor(opts...),
			// TODO: Add custom interceptors (logging, auth, etc.) here
		),
		grpc.ChainStreamInterceptor(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_validator.StreamServerInterceptor(),
			grpc_recovery.StreamServerInterceptor(opts...),
			// TODO: Add custom interceptors (logging, auth, etc.) here
		),
	)

	// Enable reflection for debugging & tools like grpc_cli
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
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	go func() {
		s.Logger.Info("gRPC server listening", "address", s.listener.Addr().String())
		if err := s.server.Serve(s.listener); err != nil {
			s.Logger.Error("gRPC server failed to serve", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server
func (s *BaseGrpcServer) Stop() {
	s.Logger.Info("Attempting to gracefully stop gRPC server...")
	s.server.GracefulStop()
	if s.listener != nil {
		s.Logger.Info("Closing gRPC listener.")
		_ = s.listener.Close() // Ignore error on close, already stopping
	}
	s.Logger.Info("gRPC server stopped.")
}

// Server returns the underlying grpc.Server instance
func (s *BaseGrpcServer) Server() *grpc.Server {
	return s.server
}

// BaseGrpcHandler defines the interface that all gRPC service handlers (implementing generated interfaces)
// should implement to register themselves with the server.
type BaseGrpcHandler interface {
	// RegisterGrpcHandlers registers all gRPC service handlers with the server.
	RegisterGrpcHandlers(server *grpc.Server)
}
