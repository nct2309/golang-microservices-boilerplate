package dto

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Global validator instance initialized once.
var validate *validator.Validate

// init initializes the package-level validator instance.
func init() {
	validate = validator.New()
	// Optional: Register custom validation functions here if needed
	// validate.RegisterValidation("customTag", customValidationFunc)
}

// ValidationErrors represents a collection of validation errors.
// It wraps the validator.ValidationErrors for a cleaner interface.
type ValidationErrors struct {
	errors []validator.FieldError
}

// Error implements the error interface.
func (ve ValidationErrors) Error() string {
	var errMsgs []string
	for _, err := range ve.errors {
		// Customize the error message format as needed
		errMsgs = append(errMsgs, fmt.Sprintf(
			"Field '%s' failed on the '%s' tag",
			err.Field(),
			err.Tag(),
		))
	}
	return strings.Join(errMsgs, "; ")
}

// GetErrors returns the underlying slice of validator.FieldError.
func (ve ValidationErrors) GetErrors() []validator.FieldError {
	return ve.errors
}

// Validate uses the pre-initialized package-level validator instance to validate a struct based on tags.
func Validate(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		// Check if the error is a validator.ValidationErrors type
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			return ValidationErrors{errors: validationErrs}
		}
		// Return other types of errors (e.g., invalid input type)
		return fmt.Errorf("validation error: %w", err)
	}
	return nil
}
