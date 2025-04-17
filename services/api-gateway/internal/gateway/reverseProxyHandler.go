package gateway

import (
	"errors"
	"fmt"
	"strings"

	user_pb "golang-microservices-boilerplate/proto/user-service"
	water_quality_pb "golang-microservices-boilerplate/proto/water-quality-service"
	"golang-microservices-boilerplate/services/api-gateway/internal/domain"
)

// setupHandlers registers gRPC-Gateway handlers for all services.
// It attempts to register all discovered services and collects errors.
// Returns a single error if one or more registrations fail.
func (g *Gateway) setupHandlers() error {
	services, err := g.discovery.GetAllServices()
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}

	// Use a slice to collect registration errors
	var registrationErrors []error

	for _, service := range services {
		var setupErr error
		switch strings.ToLower(service.Name) {
		case "user", "user-service":
			setupErr = g.setupUserServiceHandlers(service)
		case "water-quality", "water-quality-service":
			setupErr = g.setupWaterQualityServiceHandlers(service)
		// case "patient", "patient-service":
		// 	setupErr = g.setupPatientServiceHandlers(service)
		// case "appointment", "appointment-service":
		// 	setupErr = g.setupAppointmentServiceHandlers(service)
		// case "staff", "staff-service":
		// 	setupErr = g.setupStaffServiceHandlers(service)
		// Add cases for other services here
		default:
			g.logger.Warn("Unknown service discovered, skipping handler setup", "service_name", service.Name, "endpoint", service.Endpoint)
		}

		// If setupErr occurred for this specific service, log it and add to the list
		if setupErr != nil {
			// The individual setup functions already log the detailed error
			registrationErrors = append(registrationErrors, fmt.Errorf("failed to setup %s: %w", service.Name, setupErr))
			// Continue to the next service instead of returning immediately
		}
	}

	// After attempting all services, check if any errors were collected
	if len(registrationErrors) > 0 {
		// Combine errors into a single summary error (could also use a multi-error library)
		combinedError := errors.New("failed to register one or more service handlers")
		for _, regErr := range registrationErrors {
			combinedError = fmt.Errorf("%w; %w", combinedError, regErr) // Chain errors
		}
		return combinedError
	}

	// No errors encountered
	return nil
}

// setupUserServiceHandlers registers handlers for the user service
func (g *Gateway) setupUserServiceHandlers(service domain.Service) error {
	err := user_pb.RegisterUserServiceHandlerFromEndpoint(g.ctx, g.gwMux, service.Endpoint, g.opts)
	if err != nil {
		g.logger.Error("Failed to register user service handler from endpoint", "endpoint", service.Endpoint, "error", err)
		return fmt.Errorf("failed to register user service handler from endpoint %s: %w", service.Endpoint, err)
	}

	g.logger.Info("Registered gRPC-Gateway handlers via endpoint", "service", "user-service", "endpoint", service.Endpoint)
	return nil
}

// setupWaterQualityServiceHandlers registers standard and custom handlers for the water quality service
func (g *Gateway) setupWaterQualityServiceHandlers(service domain.Service) error {
	// 1. Register Standard Handlers for all methods (except potentially the upload path)
	err := water_quality_pb.RegisterWaterQualityServiceHandlerFromEndpoint(g.ctx, g.gwMux, service.Endpoint, g.opts)
	if err != nil {
		g.logger.Error("Failed to register standard water quality service handler from endpoint", "endpoint", service.Endpoint, "error", err)
		// Decide if failure here is critical. If other methods are needed, maybe return error.
		// return fmt.Errorf("failed to register standard water quality service handler from endpoint %s: %w", service.Endpoint, err)
	} else {
		g.logger.Info("Registered standard gRPC-Gateway handlers via endpoint", "service", "water-quality-service", "endpoint", service.Endpoint)
	}

	// 2. Register Custom Handlers (e.g., for binary upload)
	customErr := registerWaterQualityCustomHandlers(g.gwMux, service) // Call the function from binary_file_handler.go
	if customErr != nil {
		g.logger.Error("Failed to register custom water quality service handlers", "endpoint", service.Endpoint, "error", customErr)
		// Combine errors if both failed, or return only customErr if standard registration was okay or skipped erroring
		if err != nil {
			return fmt.Errorf("standard handler error: %w; custom handler error: %w", err, customErr)
		}
		return fmt.Errorf("failed to register custom water quality service handlers: %w", customErr)
	} else {
		g.logger.Info("Registered custom handlers (e.g., binary upload) for water quality service", "endpoint", service.Endpoint)
	}

	// Return the original standard handler registration error if it occurred and custom was okay
	// Or return nil if both succeeded.
	return err
}
