package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	coreDTO "golang-microservices-boilerplate/pkg/core/dto"
	"golang-microservices-boilerplate/pkg/core/entity"
	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/core/repository"
	"golang-microservices-boilerplate/pkg/core/types"
)

// BaseUseCase defines common operations for all use cases/services operating on entity pointers (*T)
type BaseUseCase[T entity.Entity, CreateDTO any, UpdateDTO any] interface {
	Create(ctx context.Context, dto CreateDTO) (*T, error)
	GetByID(ctx context.Context, id uuid.UUID) (*T, error)
	List(ctx context.Context, opts types.FilterOptions) (*types.PaginationResult[T], error)
	Update(ctx context.Context, id uuid.UUID, dto UpdateDTO) (*T, error)
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
	FindWithFilter(ctx context.Context, filter map[string]interface{}, opts types.FilterOptions) (*types.PaginationResult[T], error)
	Count(ctx context.Context, filter map[string]interface{}) (int64, error)

	// Bulk Operations
	CreateMany(ctx context.Context, dtos []CreateDTO) ([]*T, error)
	UpdateMany(ctx context.Context, updates map[uuid.UUID]UpdateDTO) error
	DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error
}

// BaseUseCaseImpl implements the BaseUseCase interface for entity pointers (*T)
type BaseUseCaseImpl[T entity.Entity, CreateDTO any, UpdateDTO any] struct {
	Repository repository.BaseRepository[T]
	Logger     logger.Logger
}

// NewBaseUseCase creates a new use case implementation for entity pointers (*T)
func NewBaseUseCase[T entity.Entity, CreateDTO any, UpdateDTO any](
	repository repository.BaseRepository[T],
	logger logger.Logger,
) *BaseUseCaseImpl[T, CreateDTO, UpdateDTO] {
	return &BaseUseCaseImpl[T, CreateDTO, UpdateDTO]{
		Repository: repository,
		Logger:     logger,
	}
}

// Create processes a creation request using coreDTO functions
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Create(ctx context.Context, dto CreateDTO) (*T, error) {
	// Validate DTO using coreDTO.Validate
	if err := coreDTO.Validate(dto); err != nil {
		var validationErrs coreDTO.ValidationErrors
		if errors.As(err, &validationErrs) {
			uc.Logger.Warn("DTO validation failed", "errors", validationErrs.Error())
			return nil, NewUseCaseError(ErrInvalidInput, validationErrs.Error())
		}
		uc.Logger.Error("Validation setup error", "error", err)
		return nil, NewUseCaseError(ErrInternal, fmt.Sprintf("validation error: %v", err))
	}

	// Convert DTO to entity pointer using coreDTO.MapDTOToEntity
	var entityPtr T // Declare entity of type T
	if err := coreDTO.MapToEntity(dto, &entityPtr); err != nil {
		uc.Logger.Error("Failed to map DTO to entity", "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to process input data mapping")
	}

	// Create entity in repository
	if err := uc.Repository.Create(ctx, &entityPtr); err != nil {
		uc.Logger.Error("Failed to create entity in repository", "error", err)
		// Consider checking for specific DB errors (e.g., unique constraint)
		return nil, err // Return original repository error
	}

	return &entityPtr, nil
}

// GetByID retrieves an entity by its ID
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) GetByID(ctx context.Context, id uuid.UUID) (*T, error) {
	entityPtr, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		// Specific handling for not found remains, but others are passed through
		if err.Error() == "entity not found" { // Match error from repository
			return nil, NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found", id))
		}
		uc.Logger.Error("Failed to get entity by ID", "id", id, "error", err)
		return nil, err // Return original repository error
	}
	return entityPtr, nil
}

// List retrieves all entities with pagination
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) List(ctx context.Context, opts types.FilterOptions) (*types.PaginationResult[T], error) {
	result, err := uc.Repository.FindAll(ctx, opts)
	if err != nil {
		uc.Logger.Error("Failed to list entities", "error", err)
		return nil, err // Return original repository error
	}
	return result, nil
}

// Update modifies an existing entity using coreDTO functions
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Update(ctx context.Context, id uuid.UUID, dto UpdateDTO) (*T, error) {
	// Validate DTO using coreDTO.Validate
	if err := coreDTO.Validate(dto); err != nil {
		var validationErrs coreDTO.ValidationErrors
		if errors.As(err, &validationErrs) {
			uc.Logger.Warn("Update DTO validation failed", "id", id, "errors", validationErrs.Error())
			return nil, NewUseCaseError(ErrInvalidInput, validationErrs.Error())
		}
		uc.Logger.Error("Update validation setup error", "id", id, "error", err)
		return nil, NewUseCaseError(ErrInternal, fmt.Sprintf("validation error: %v", err))
	}

	// Fetch the existing entity pointer
	entityPtr, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		// Specific handling for not found remains
		if err.Error() == "entity not found" {
			return nil, NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found for update", id))
		}
		uc.Logger.Error("Failed to get entity for update", "id", id, "error", err)
		return nil, err // Return original repository error
	}

	// Apply updates from DTO to the existing entity pointer using coreDTO.MapDTOToEntity
	if err := coreDTO.MapToEntity(dto, entityPtr); err != nil {
		uc.Logger.Error("Failed to map update DTO to entity", "id", id, "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to apply updates mapping") // Keep internal error for mapping issues
	}

	// Save the updated entity
	if err := uc.Repository.Update(ctx, entityPtr); err != nil {
		uc.Logger.Error("Failed to update entity in repository", "id", id, "error", err)
		// Consider checking for specific DB errors
		return nil, err // Return original repository error
	}

	return entityPtr, nil
}

// Delete soft-deletes or hard-deletes an entity based on the flag
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	// Check if entity exists first to provide a NotFound error if it doesn't
	_, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if err.Error() == "entity not found" {
			return NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found for deletion", id))
		}
		uc.Logger.Error("Failed to find entity for deletion", "id", id, "hardDelete", hardDelete, "error", err)
		return err // Return original repository error
	}

	// Perform delete (soft or hard)
	if err := uc.Repository.Delete(ctx, id, hardDelete); err != nil {
		uc.Logger.Error("Failed to delete entity", "id", id, "hardDelete", hardDelete, "error", err)
		return err // Return original repository error
	}

	return nil
}

// FindWithFilter retrieves entities with a filter and pagination
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) FindWithFilter(
	ctx context.Context,
	filter map[string]interface{},
	opts types.FilterOptions,
) (*types.PaginationResult[T], error) {
	result, err := uc.Repository.FindWithFilter(ctx, filter, opts)
	if err != nil {
		uc.Logger.Error("Failed to find entities with filter", "error", err)
		return nil, err // Return original repository error
	}
	return result, nil
}

// Count returns the count of entities matching the filter
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	count, err := uc.Repository.Count(ctx, filter)
	if err != nil {
		uc.Logger.Error("Failed to count entities", "error", err)
		return 0, err // Return original repository error
	}
	return count, nil
}

// --- Bulk Operations Implementation ---

// CreateMany processes a bulk creation request using coreDTO functions
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) CreateMany(ctx context.Context, dtos []CreateDTO) ([]*T, error) {
	if len(dtos) == 0 {
		return []*T{}, nil
	}

	entities := make([]*T, 0, len(dtos))
	for i, dto := range dtos {
		// Validate DTO
		if err := coreDTO.Validate(dto); err != nil {
			// Simplified error handling for bulk, could collect all errors
			return nil, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("validation error for item %d: %v", i, err))
		}

		// Convert DTO to entity pointer
		var entityPtr T
		if err := coreDTO.MapToEntity(dto, &entityPtr); err != nil {
			uc.Logger.Error("Failed to map DTO to entity for bulk create", "index", i, "error", err)
			return nil, NewUseCaseError(ErrInternal, fmt.Sprintf("failed to process input data mapping for item %d", i))
		}
		entities = append(entities, &entityPtr)
	}

	// Create entities in repository
	if err := uc.Repository.CreateMany(ctx, entities); err != nil {
		uc.Logger.Error("Failed to bulk create entities", "error", err)
		return nil, err // Return original repository error
	}

	return entities, nil
}

// UpdateMany processes a bulk update request.
// It fetches each entity, validates/applies the DTO, and calls the repository's UpdateMany.
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) UpdateMany(ctx context.Context, updates map[uuid.UUID]UpdateDTO) error {
	if len(updates) == 0 {
		return nil // Nothing to update
	}

	updatedEntities := make([]*T, 0, len(updates))
	for id, dto := range updates {
		// Validate DTO
		if err := coreDTO.Validate(dto); err != nil {
			// Consider collecting all errors instead of returning on the first one
			return NewUseCaseError(ErrInvalidInput, fmt.Sprintf("validation error for ID %s: %v", id, err))
		}

		// Fetch the existing entity
		entityPtr, err := uc.Repository.FindByID(ctx, id)
		if err != nil {
			if err.Error() == "entity not found" {
				return NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found for bulk update", id))
			}
			uc.Logger.Error("Failed to get entity for bulk update", "id", id, "error", err)
			return err // Return original repository error
		}

		// Apply updates from DTO to the existing entity pointer
		if err := coreDTO.MapToEntity(dto, entityPtr); err != nil {
			uc.Logger.Error("Failed to map update DTO to entity for bulk update", "id", id, "error", err)
			return NewUseCaseError(ErrInternal, fmt.Sprintf("failed to apply updates mapping for ID %s", id))
		}

		updatedEntities = append(updatedEntities, entityPtr)
	}

	// Call repository's UpdateMany with the prepared entities
	if err := uc.Repository.UpdateMany(ctx, updatedEntities); err != nil {
		uc.Logger.Error("Failed to bulk update entities in repository", "count", len(updatedEntities), "error", err)
		return err // Return original repository error
	}

	return nil
}

// DeleteMany soft-deletes or hard-deletes entities matching the provided IDs.
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error {
	if len(ids) == 0 {
		return nil // Nothing to delete
	}

	// We might want to check if all IDs exist first, but that could be expensive.
	// The repository level might handle non-existent IDs gracefully (e.g., deleting those that exist).
	// Alternatively, add a check here if strict existence is required.

	if err := uc.Repository.DeleteMany(ctx, ids, hardDelete); err != nil {
		uc.Logger.Error("Failed to bulk delete entities", "count", len(ids), "hardDelete", hardDelete, "error", err)
		return err // Return original repository error
	}
	return nil
}

// UseCaseErrorType defines the type of error
type UseCaseErrorType string

const (
	ErrNotFound     UseCaseErrorType = "not_found"
	ErrInvalidInput UseCaseErrorType = "invalid_input"
	ErrUnauthorized UseCaseErrorType = "unauthorized"
	ErrForbidden    UseCaseErrorType = "forbidden"
	ErrConflict     UseCaseErrorType = "conflict"
	ErrInternal     UseCaseErrorType = "internal_error"
)

// UseCaseError represents an error from a use case
type UseCaseError struct {
	Type    UseCaseErrorType
	Message string
}

// Error returns the error message
func (e *UseCaseError) Error() string {
	return e.Message
}

// NewUseCaseError creates a new use case error
func NewUseCaseError(errorType UseCaseErrorType, message string) error {
	return &UseCaseError{
		Type:    errorType,
		Message: message,
	}
}
