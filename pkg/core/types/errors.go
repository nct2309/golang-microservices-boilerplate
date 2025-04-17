package types

import "errors"

// Standard application errors
var (
	ErrNotFound     = errors.New("resource not found")
	ErrDatabase     = errors.New("database error")
	ErrValidation   = errors.New("validation error")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("resource conflict") // e.g., duplicate entry
	ErrStorage      = errors.New("storage error")     // For file/object storage issues
	ErrExternal     = errors.New("external service error")
	ErrRateLimit    = errors.New("rate limit exceeded")
	ErrInternal     = errors.New("internal server error")
)
