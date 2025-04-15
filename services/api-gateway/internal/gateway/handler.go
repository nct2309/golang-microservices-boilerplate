package gateway

import (
	"errors"
	"fmt"
	"strings"

	user_pb "golang-microservices-boilerplate/proto/user-service"
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

// func (g *Gateway) setupPatientServiceHandlers(service domain.Service) error {
// 	err := patient_pb.RegisterPatientServiceHandlerFromEndpoint(g.ctx, g.gwMux, service.Endpoint, g.opts)
// 	if err != nil {
// 		g.logger.Error("Failed to register patient service handler from endpoint", "endpoint", service.Endpoint, "error", err)
// 		return fmt.Errorf("failed to register patient service handler from endpoint %s: %w", service.Endpoint, err)
// 	}
// 	g.logger.Info("Registered gRPC-Gateway handlers via endpoint", "service", "patient-service", "endpoint", service.Endpoint)
// 	return nil
// }

// func (g *Gateway) setupAppointmentServiceHandlers(service domain.Service) error {
// 	err := appointment_pb.RegisterAppointmentServiceHandlerFromEndpoint(g.ctx, g.gwMux, service.Endpoint, g.opts)
// 	if err != nil {
// 		g.logger.Error("Failed to register appointment service handler from endpoint", "endpoint", service.Endpoint, "error", err)
// 		return fmt.Errorf("failed to register appointment service handler from endpoint %s: %w", service.Endpoint, err)
// 	}
// 	g.logger.Info("Registered gRPC-Gateway handlers via endpoint", "service", "appointment-service", "endpoint", service.Endpoint)
// 	return nil
// }

// func (g *Gateway) setupStaffServiceHandlers(service domain.Service) error {
// 	err := staff_pb.RegisterStaffServiceHandlerFromEndpoint(g.ctx, g.gwMux, service.Endpoint, g.opts)
// 	if err != nil {
// 		g.logger.Error("Failed to register staff service handler from endpoint", "endpoint", service.Endpoint, "error", err)
// 		return fmt.Errorf("failed to register staff service handler from endpoint %s: %w", service.Endpoint, err)
// 	}
// 	g.logger.Info("Registered gRPC-Gateway handlers via endpoint", "service", "staff-service", "endpoint", service.Endpoint)
// 	return nil
// }
