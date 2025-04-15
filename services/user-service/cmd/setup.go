package main

import (
	"log"
	"time"

	"golang-microservices-boilerplate/pkg/core/database"
	"golang-microservices-boilerplate/pkg/core/grpc"
	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/utils"
	controller "golang-microservices-boilerplate/services/user-service/internal/controller"
	entity "golang-microservices-boilerplate/services/user-service/internal/entity"
	"golang-microservices-boilerplate/services/user-service/internal/repository"
	"golang-microservices-boilerplate/services/user-service/internal/usecase"
)

// SetupServices initializes all the services needed by the application
func SetupServices() (*grpc.BaseGrpcServer, error) {
	// Initialize logger
	logConfig := logger.LoadLogConfigFromEnv()
	logConfig.AppName = utils.GetEnv("SERVER_APP_NAME", "User Service")
	appLogger, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, err
	}

	appLogger.Info("Setting up user service")

	// Initialize database connection
	db, err := database.NewDatabaseConnection(database.DefaultDBConfig())
	if err != nil {
		appLogger.Error("Failed to connect to database", "error", err)
		return nil, err
	}
	appLogger.Info("Connected to database")

	// Auto migrate models
	if err := db.MigrateModels(&entity.User{}); err != nil {
		appLogger.Error("Failed to auto-migrate models", "error", err)
		return nil, err
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)

	// Token generation durations
	accessTokenDuration := 7 * 24 * time.Hour   // Example: 7 days
	refreshTokenDuration := 30 * 24 * time.Hour // Example: 30 days

	// Initialize use cases with all required arguments
	userUseCase := usecase.NewUserUseCase(userRepo, appLogger, &accessTokenDuration, &refreshTokenDuration)

	// Initialize mapper
	userMapper := controller.NewUserMapper()

	// Initialize gRPC server with interceptors
	grpcServer := grpc.NewBaseGrpcServer(appLogger)

	// Register the service implementation with the gRPC server
	controller.RegisterUserServiceServer(grpcServer.Server(), userUseCase, userMapper)

	log.Printf("User service setup completed successfully")
	return grpcServer, nil
}
