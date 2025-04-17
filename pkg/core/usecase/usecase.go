package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"golang-microservices-boilerplate/pkg/core/entity"
	"golang-microservices-boilerplate/pkg/core/logger"
	"golang-microservices-boilerplate/pkg/core/repository"
	"golang-microservices-boilerplate/pkg/core/types"
)

// BaseUseCase defines common operations for all use cases/services operating on entity pointers (*T)
type BaseUseCase[T entity.Entity] interface {
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id uuid.UUID) (*T, error)
	List(ctx context.Context, opts types.FilterOptions) (*types.PaginationResult[T], error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
	FindWithFilter(ctx context.Context, filter map[string]interface{}, opts types.FilterOptions) (*types.PaginationResult[T], error)
	Count(ctx context.Context, filter map[string]interface{}) (int64, error)

	// Bulk Operations
	CreateMany(ctx context.Context, entities []*T) ([]*T, error)
	UpdateMany(ctx context.Context, entities []*T) ([]*T, error)
	DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error
}

// BaseUseCaseImpl implements the BaseUseCase interface for entity pointers (*T)
type BaseUseCaseImpl[T entity.Entity] struct {
	Repository repository.BaseRepository[T]
	Logger     logger.Logger
}

// NewBaseUseCase creates a new use case implementation for entity pointers (*T)
func NewBaseUseCase[T entity.Entity](
	repository repository.BaseRepository[T],
	logger logger.Logger,
) *BaseUseCaseImpl[T] {
	return &BaseUseCaseImpl[T]{
		Repository: repository,
		Logger:     logger,
	}
}

// Create processes a creation request using the provided entity pointer
func (uc *BaseUseCaseImpl[T]) Create(ctx context.Context, entityPtr *T) error {
	// Validation should now happen before calling this method, or rely on entity hooks (e.g., BeforeCreate)
	// Mapping from external data (e.g., proto) should also happen before calling this method.

	// Create entity in repository
	if err := uc.Repository.Create(ctx, entityPtr); err != nil {
		uc.Logger.Error("Failed to create entity in repository", "entityType", fmt.Sprintf("%T", entityPtr), "error", err)
		// Consider checking for specific DB errors (e.g., unique constraint)
		return err // Return original repository error
	}

	// The entityPtr is modified in place by the repository (e.g., ID set)
	return nil
}

// GetByID retrieves an entity by its ID
func (uc *BaseUseCaseImpl[T]) GetByID(ctx context.Context, id uuid.UUID) (*T, error) {
	entityPtr, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if err.Error() == "entity not found" { // Example error string check
			return nil, NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found", id))
		}
		uc.Logger.Error("Failed to get entity by ID", "id", id, "error", err)
		return nil, err // Return original repository error
	}
	return entityPtr, nil
}

// List retrieves all entities with pagination
func (uc *BaseUseCaseImpl[T]) List(ctx context.Context, opts types.FilterOptions) (*types.PaginationResult[T], error) {
	result, err := uc.Repository.FindAll(ctx, opts)
	if err != nil {
		uc.Logger.Error("Failed to list entities", "error", err)
		return nil, err // Return original repository error
	}
	return result, nil
}

// Update modifies an existing entity based on the provided entity pointer.
func (uc *BaseUseCaseImpl[T]) Update(ctx context.Context, entityPtr *T) error {
	// Validation and mapping should happen before calling this method, or rely on entity hooks (e.g., BeforeUpdate).
	// The caller is responsible for providing the full entity state to be saved.

	// Ensure the entity pointer is valid and has an ID before proceeding
	var entityID uuid.UUID
	if entityPtr == nil {
		uc.Logger.Warn("Update called with nil entity pointer")
		return NewUseCaseError(ErrInvalidInput, "cannot update nil entity")
	}
	entityID = (*entityPtr).GetID()
	if entityID == uuid.Nil {
		uc.Logger.Warn("Update called with entity having nil ID")
		return NewUseCaseError(ErrInvalidInput, "cannot update entity with nil ID")
	}

	// Save the updated entity using Update()
	// Repository's Update should handle finding the record by ID from entityPtr and updating it.
	if err := uc.Repository.Update(ctx, entityPtr); err != nil {
		if err.Error() == "entity not found" { // Example check if repository.Update returns not found
			uc.Logger.Warn("Attempted to update non-existent entity", "id", entityID.String())
			return NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found for update", entityID.String()))
		}
		uc.Logger.Error("Failed to update entity in repository", "id", entityID.String(), "error", err)
		// Consider checking for specific DB errors
		return err // Return original repository error
	}

	// The entityPtr reflects the state after the update (if the repository modifies it)
	return nil
}

// Delete soft-deletes or hard-deletes an entity based on the flag
func (uc *BaseUseCaseImpl[T]) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	// Check if entity exists first to provide a NotFound error if it doesn't
	_, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if err.Error() == "entity not found" { // Example error string check
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
func (uc *BaseUseCaseImpl[T]) FindWithFilter(
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
func (uc *BaseUseCaseImpl[T]) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	count, err := uc.Repository.Count(ctx, filter)
	if err != nil {
		uc.Logger.Error("Failed to count entities", "error", err)
		return 0, err // Return original repository error
	}
	return count, nil
}

// --- Bulk Operations Implementation ---

// CreateMany processes a bulk creation request using the provided entity pointers
// Returns the created entities (with IDs populated)
func (uc *BaseUseCaseImpl[T]) CreateMany(ctx context.Context, entities []*T) ([]*T, error) {
	if len(entities) == 0 {
		return entities, nil
	}
	// Validation should happen before calling, or rely on entity hooks.

	// Create entities in repository, capture the returned slice
	createdEntities, err := uc.Repository.CreateMany(ctx, entities)
	if err != nil {
		uc.Logger.Error("Failed to bulk create entities", "count", len(entities), "error", err)
		return nil, err // Return nil slice on error
	}

	// Return the entities populated by the repository
	return createdEntities, nil
}

// UpdateMany processes a bulk update request using the provided entity pointers.
// Returns the fully updated entities fetched from the repository after the update.
func (uc *BaseUseCaseImpl[T]) UpdateMany(ctx context.Context, entities []*T) ([]*T, error) {
	if len(entities) == 0 {
		return entities, nil // Nothing to update
	}
	// Validation should happen before calling, or rely on entity hooks.
	// Ensure entities are valid before passing them?
	for i, entityPtr := range entities {
		if entityPtr == nil || (*entityPtr).GetID() == uuid.Nil {
			uc.Logger.Warn("UpdateMany called with nil entity or entity with nil ID", "index", i)
			return nil, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("invalid entity at index %d for bulk update", i))
		}
	}

	// Call repository's UpdateMany, capture the returned updated entities
	updatedEntities, err := uc.Repository.UpdateMany(ctx, entities)
	if err != nil {
		uc.Logger.Error("Failed to bulk update entities in repository", "count", len(entities), "error", err)
		return nil, err // Return nil slice on error
	}

	return updatedEntities, nil
}

// DeleteMany soft-deletes or hard-deletes entities matching the provided IDs.
func (uc *BaseUseCaseImpl[T]) DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error {
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
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewUseCaseError creates a new use case error
func NewUseCaseError(errorType UseCaseErrorType, message string) error {
	return &UseCaseError{
		Type:    errorType,
		Message: message,
	}
}
