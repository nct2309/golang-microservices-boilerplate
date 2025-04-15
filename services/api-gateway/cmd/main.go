package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/utils"
	"golang-microservices-boilerplate/services/api-gateway/internal/gateway"
	"golang-microservices-boilerplate/services/api-gateway/internal/infrastructure/adapter"
	"golang-microservices-boilerplate/services/api-gateway/internal/infrastructure/k8s"
)

func main() {
	// Load environment variables
	if err := utils.LoadEnv(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Setup structured logger
	logger, err := logger.NewLoggerFromEnv()
	if err != nil {
		// If we can't create a structured logger, fall back to standard logger
		stdLogger := log.New(os.Stdout, "[API-GATEWAY] ", log.LstdFlags|log.Lshortfile)
		stdLogger.Printf("Failed to initialize structured logger: %v, falling back to standard logger", err)

		// Create adapter with the standard logger
		logger = adapter.NewStdLoggerAdapter("[API-GATEWAY] ")
	}

	appLogger := logger.Named("api-gateway")
	appLogger.Info("Starting API Gateway...")

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Kubernetes service discovery
	namespace := utils.GetEnv("K8S_NAMESPACE", "ride-sharing")

	discovery, err := k8s.NewKubernetesDiscovery(
		k8s.WithNamespace(namespace),
		k8s.WithLogger(log.New(os.Stdout, "[K8S-DISCOVERY] ", log.LstdFlags)), // Keep using std logger for k8s for now
	)
	if err != nil {
		appLogger.Fatal("Failed to initialize service discovery", "error", err)
	}
	defer discovery.Close()

	// Initialize gateway
	gw := gateway.NewGateway(
		ctx,
		discovery,
		gateway.WithLogger(logger.Named("gateway")),
	)

	// Start server in a goroutine
	port := utils.GetEnv("PORT", "8081")
	go func() {
		if err := gw.Start(port); err != nil {
			appLogger.Fatal("Failed to start server", "error", err)
		}
	}()

	appLogger.Info("API Gateway listening", "port", port)

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down API Gateway...")

	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown the gateway
	if err := gw.Shutdown(shutdownCtx); err != nil {
		appLogger.Fatal("Gateway shutdown failed", "error", err)
	}

	appLogger.Info("API Gateway stopped")
}
