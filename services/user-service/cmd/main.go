package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang-microservices-boilerplate/pkg/core/database"
	"golang-microservices-boilerplate/pkg/core/grpc"
	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/utils"
	pb "golang-microservices-boilerplate/proto/user-service" // Import generated proto package
	controller "golang-microservices-boilerplate/services/user-service/internal/controller"
	entity "golang-microservices-boilerplate/services/user-service/internal/model/entity"
	"golang-microservices-boilerplate/services/user-service/internal/repository"
	"golang-microservices-boilerplate/services/user-service/internal/usecase"
)

// Placeholder TokenGenerator implementation
type simpleTokenGenerator struct{}

func (tg *simpleTokenGenerator) GenerateTokenPair(customClaims map[string]interface{}, accessDuration, refreshDuration time.Duration) (accessToken, refreshToken string, expiresAt int64, err error) {
	// In a real implementation, this would generate actual JWTs
	log.Printf("Generating placeholder tokens with claims: %v, accessDur: %v, refreshDur: %v", customClaims, accessDuration, refreshDuration)
	accessToken = utils.GetEnv("ACCESS_TOKEN_SECRET", "access_token_secret_wqim")
	refreshToken = utils.GetEnv("REFRESH_TOKEN_SECRET", "refresh_token_secret_KMT")
	expiresAt = time.Now().Add(accessDuration).Unix()
	return
}

func main() {
	// Load environment variables
	if err := utils.LoadEnv(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// Initialize logger
	logConfig := logger.LoadLogConfigFromEnv()
	logConfig.AppName = utils.GetEnv("SERVER_APP_NAME", "User Service")
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("Starting user service")

	// Initialize database connection
	db, err := database.NewDatabaseConnection(database.DefaultDBConfig())
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	logger.Info("Connected to database")

	// Auto migrate models (using entity package)
	if err := db.MigrateModels(&entity.User{}); err != nil {
		logger.Fatal("Failed to auto-migrate models", "error", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)

	// Initialize Token Generator and Durations
	tokenGen := &simpleTokenGenerator{}
	// TODO: Load durations from config/env
	accessTokenDuration := 7 * 24 * time.Hour   // Example: 7 days
	refreshTokenDuration := 30 * 24 * time.Hour // Example: 30 days

	// Initialize use cases with all required arguments
	userUseCase := usecase.NewUserUseCase(userRepo, logger, tokenGen, &accessTokenDuration, &refreshTokenDuration)

	// Initialize gRPC server with interceptors
	grpcServer := grpc.NewBaseGrpcServer(logger)

	// Initialize gRPC service implementation (the controller)
	userServer := controller.NewUserServer(userUseCase) // Controller now acts as the server implementation

	// Register the service implementation with the gRPC server
	pb.RegisterUserServiceServer(grpcServer.Server(), userServer) // Use generated registration function

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
