package internal

import (
	"context"
	"errors"
)

// CommonError defines standard error types for use cases
type CommonError string

const (
	// ErrNotFound indicates a requested resource doesn't exist
	ErrNotFound CommonError = "resource_not_found"

	// ErrInvalidInput indicates the input data failed validation
	ErrInvalidInput CommonError = "invalid_input"

	// ErrUnauthorized indicates the action is not permitted for the current user
	ErrUnauthorized CommonError = "unauthorized"

	// ErrForbidden indicates the action is forbidden even with authentication
	ErrForbidden CommonError = "forbidden"

	// ErrInternal indicates an internal server error occurred
	ErrInternal CommonError = "internal_error"

	// ErrConflict indicates a conflict with the current state of the resource
	ErrConflict CommonError = "conflict"
)

// Error makes CommonError implement the error interface
func (e CommonError) Error() string {
	return string(e)
}

// UseCaseError wraps errors from use cases with additional context
type UseCaseError struct {
	Type    CommonError
	Message string
	Cause   error
}

// Error implements the error interface for UseCaseError
func (e *UseCaseError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Type)
}

// Unwrap returns the underlying error
func (e *UseCaseError) Unwrap() error {
	return e.Cause
}

// NewUseCaseError creates a new UseCaseError
func NewUseCaseError(errType CommonError, message string, cause error) *UseCaseError {
	return &UseCaseError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// BaseUseCase defines common methods for all use cases
type BaseUseCase interface {
	// Validate validates the input data for the use case
	Validate(ctx context.Context, input interface{}) error

	// Execute performs the main business logic of the use case
	Execute(ctx context.Context, input interface{}) (interface{}, error)
}

// BaseUseCaseImpl provides a basic implementation of BaseUseCase
type BaseUseCaseImpl struct {
	// This can contain common dependencies like logging, metrics, etc.
	Logger Logger
}

// NewBaseUseCase creates a new instance of BaseUseCaseImpl
func NewBaseUseCase(logger Logger) *BaseUseCaseImpl {
	return &BaseUseCaseImpl{
		Logger: logger,
	}
}

// Validate is a default implementation that always returns nil
// Should be overridden by specific use cases to implement actual validation
func (uc *BaseUseCaseImpl) Validate(ctx context.Context, input interface{}) error {
	// Default implementation does no validation
	// Child use cases should override this method
	return nil
}

// Execute provides a default implementation that returns an error
// Should be overridden by specific use cases to implement actual business logic
func (uc *BaseUseCaseImpl) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	// This is just a placeholder that should be overridden
	return nil, errors.New("execute method not implemented")
}

// Logger is an interface for logging operations
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// WithTransaction is an optional interface that use cases can implement
// if they need transaction support
type WithTransaction interface {
	WithTx(tx interface{}) BaseUseCase
}
