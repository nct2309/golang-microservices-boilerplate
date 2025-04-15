package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang-microservices-boilerplate/pkg/utils"
)

func main() {
	// Load environment variables
	if err := utils.LoadEnv(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// Setup all services
	grpcServer, err := SetupServices()
	if err != nil {
		log.Fatalf("Failed to setup services: %v", err)
	}

	// Start gRPC server
	if err := grpcServer.Start(); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}
	log.Printf("gRPC server started successfully at %s:%s\n", grpcServer.Config.Host, grpcServer.Config.Port)

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	grpcServer.Stop()
	log.Println("Server gracefully stopped")
}
