package internal

import (
	"errors"
	"net/http"
	"reflect"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// BaseController provides common functionality for all controllers
type BaseController struct {
	Logger Logger
}

// NewBaseController creates a new instance of BaseController
func NewBaseController(logger Logger) *BaseController {
	return &BaseController{
		Logger: logger,
	}
}

// ResponseError represents the standard error response structure
type ResponseError struct {
	Error       string `json:"error"`
	Message     string `json:"message"`
	StatusCode  int    `json:"status_code"`
	RequestID   string `json:"request_id,omitempty"`
	InvalidData any    `json:"invalid_data,omitempty"`
}

// ResponseSuccess represents the standard success response structure
type ResponseSuccess struct {
	Data       any             `json:"data,omitempty"`
	Message    string          `json:"message,omitempty"`
	StatusCode int             `json:"status_code"`
	RequestID  string          `json:"request_id,omitempty"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

// PaginationMeta provides metadata for paginated responses
type PaginationMeta struct {
	Page      int `json:"page"`
	PerPage   int `json:"per_page"`
	TotalRows int `json:"total_rows"`
	TotalPage int `json:"total_page"`
}

// BindJSON binds request body to the given struct and handles validation
func (bc *BaseController) BindJSON(c *fiber.Ctx, obj interface{}) error {
	// Parse request body
	if err := c.BodyParser(obj); err != nil {
		bc.Logger.Error("Failed to parse request body", "error", err)
		return bc.RespondWithError(c, http.StatusBadRequest, "invalid_request", "Failed to parse request body", nil)
	}

	// Additional validation can be performed here if needed
	return nil
}

// ExtractPathParam extracts and parses path parameters
func (bc *BaseController) ExtractPathParam(c *fiber.Ctx, paramName string, target interface{}) error {
	param := c.Params(paramName)
	if param == "" {
		return bc.RespondWithError(c, http.StatusBadRequest, "invalid_parameter", "Required path parameter is missing", nil)
	}

	// Handle specific parameter types
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}

	targetElem := targetValue.Elem()
	switch targetElem.Kind() {
	case reflect.String:
		targetElem.SetString(param)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// You would need to implement int parsing logic here

	default:
		// Handle UUID if we're targeting uuid.UUID
		if targetElem.Type() == reflect.TypeOf(uuid.UUID{}) {
			id, err := uuid.Parse(param)
			if err != nil {
				return bc.RespondWithError(c, http.StatusBadRequest, "invalid_id", "Invalid ID format", nil)
			}
			targetElem.Set(reflect.ValueOf(id))
		}
	}

	return nil
}

// RespondWithError sends a standardized error response
func (bc *BaseController) RespondWithError(c *fiber.Ctx, statusCode int, errorType string, message string, invalidData any) error {
	resp := ResponseError{
		Error:       errorType,
		Message:     message,
		StatusCode:  statusCode,
		RequestID:   c.GetRespHeader("X-Request-ID", ""),
		InvalidData: invalidData,
	}

	bc.Logger.Error(message,
		"status_code", statusCode,
		"error", errorType,
		"request_id", resp.RequestID,
	)

	return c.Status(statusCode).JSON(resp)
}

// RespondWithSuccess sends a standardized success response
func (bc *BaseController) RespondWithSuccess(c *fiber.Ctx, statusCode int, data any, message string, pagination *PaginationMeta) error {
	resp := ResponseSuccess{
		Data:       data,
		Message:    message,
		StatusCode: statusCode,
		RequestID:  c.GetRespHeader("X-Request-ID", ""),
		Pagination: pagination,
	}

	return c.Status(statusCode).JSON(resp)
}

// HandleUseCaseError properly maps use case errors to HTTP responses
func (bc *BaseController) HandleUseCaseError(c *fiber.Ctx, err error) error {
	// Check if it's our custom error type
	var useCaseError *UseCaseError
	if errors.As(err, &useCaseError) {
		switch useCaseError.Type {
		case ErrNotFound:
			return bc.RespondWithError(c, http.StatusNotFound, "not_found", useCaseError.Error(), nil)
		case ErrInvalidInput:
			return bc.RespondWithError(c, http.StatusBadRequest, "invalid_input", useCaseError.Error(), nil)
		case ErrUnauthorized:
			return bc.RespondWithError(c, http.StatusUnauthorized, "unauthorized", useCaseError.Error(), nil)
		case ErrForbidden:
			return bc.RespondWithError(c, http.StatusForbidden, "forbidden", useCaseError.Error(), nil)
		case ErrConflict:
			return bc.RespondWithError(c, http.StatusConflict, "conflict", useCaseError.Error(), nil)
		case ErrInternal:
			return bc.RespondWithError(c, http.StatusInternalServerError, "internal_error", "An internal error occurred", nil)
		}
	}

	// Default case for unexpected errors
	bc.Logger.Error("Unhandled error", "error", err)
	return bc.RespondWithError(c, http.StatusInternalServerError, "internal_error", "An internal error occurred", nil)
}
