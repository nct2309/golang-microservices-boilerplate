package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang-microservices-boilerplate/pkg/core"
	"golang-microservices-boilerplate/pkg/utils"
	"golang-microservices-boilerplate/services/user-service/internal"
)

func main() {
	// Load environment variables
	if err := utils.LoadEnv(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// Initialize logger
	logConfig := core.LoadLogConfigFromEnv()
	logConfig.AppName = utils.GetEnv("SERVER_APP_NAME", "User Service")
	logger, err := core.NewLogger(logConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("Starting user service")

	// Initialize database connection
	db, err := core.NewDatabaseConnection(core.DefaultDBConfig())
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	logger.Info("Connected to database")

	// Auto migrate models
	if err := db.MigrateModels(&internal.User{}); err != nil {
		logger.Fatal("Failed to auto-migrate models", "error", err)
	}

	// Initialize repositories
	userRepo := internal.NewUserRepository(db.DB)

	// Initialize use cases
	userUseCase := internal.NewUserUseCase(userRepo, logger)

	// Initialize gRPC server with interceptors
	grpcServer := core.NewBaseGrpcServer(logger)

	// Initialize gRPC handlers
	userController := internal.NewUserController(userUseCase, logger)

	// Register handlers with the gRPC server
	userController.RegisterGrpcHandlers(grpcServer.Server())

	// Start gRPC server
	if err := grpcServer.Start(); err != nil {
		logger.Fatal("Failed to start gRPC server", "error", err)
	}
	logger.Info("gRPC server started successfully", "address", grpcServer.Config.Host+":"+grpcServer.Config.Port)

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	grpcServer.Stop()
	logger.Info("Server gracefully stopped")
}
