package controller

import (
	"errors"
	"fmt"
	"golang-microservices-boilerplate/pkg/core/usecase"
	"net/http"

	"google.golang.org/grpc/status"
)

func MapErrorToHttpStatus(err error) error {
	var ucErr *usecase.UseCaseError
	if errors.As(err, &ucErr) {
		switch ucErr.Type {
		case usecase.ErrNotFound:
			return status.Error(http.StatusNotFound, ucErr.Message)
		case usecase.ErrInvalidInput:
			return status.Error(http.StatusBadRequest, ucErr.Message)
		case usecase.ErrConflict:
			return status.Error(http.StatusConflict, ucErr.Message)
		case usecase.ErrInternal:
			return status.Error(http.StatusInternalServerError, ucErr.Message)
		case usecase.ErrUnauthorized:
			return status.Error(http.StatusUnauthorized, ucErr.Message)
		case usecase.ErrForbidden:
			return status.Error(http.StatusForbidden, ucErr.Message)
		default:
			return status.Error(http.StatusInternalServerError, fmt.Sprintf("an unexpected error occurred: %v", ucErr.Message))
		}
	}
	return status.Error(http.StatusInternalServerError, fmt.Sprintf("an unexpected error occurred: %v", ucErr.Message))
}
