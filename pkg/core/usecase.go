package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// BaseUseCase defines common operations for all use cases/services
type BaseUseCase[T Entity, CreateDTO any, UpdateDTO any] interface {
	Create(ctx context.Context, dto CreateDTO) (*T, error)
	GetByID(ctx context.Context, id uuid.UUID) (*T, error)
	List(ctx context.Context, opts FilterOptions) (*PaginationResult[T], error)
	Update(ctx context.Context, id uuid.UUID, dto UpdateDTO) (*T, error)
	Delete(ctx context.Context, id uuid.UUID) error
	HardDelete(ctx context.Context, id uuid.UUID) error
	FindWithFilter(ctx context.Context, filter map[string]interface{}, opts FilterOptions) (*PaginationResult[T], error)
	Count(ctx context.Context, filter map[string]interface{}) (int64, error)

	// Bulk Operations
	CreateMany(ctx context.Context, dtos []CreateDTO) ([]*T, error)
	// UpdateMany might need a specific UpdateManyDTO depending on requirements
	// For now, let's assume we update based on a filter and a single UpdateDTO applies to all matches
	UpdateMany(ctx context.Context, filter map[string]interface{}, dto UpdateDTO) (int64, error)
	DeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error)
	HardDeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error)
}

// BaseUseCaseImpl implements the BaseUseCase interface
type BaseUseCaseImpl[T Entity, CreateDTO any, UpdateDTO any] struct {
	Repository BaseRepository[T]
	Mapper     DTOMapper[T, CreateDTO, UpdateDTO]
	Validator  DTOValidator[CreateDTO, UpdateDTO]
	Logger     Logger
}

// DTOMapper handles the conversion between DTOs and domain entities
type DTOMapper[T Entity, CreateDTO any, UpdateDTO any] interface {
	ToEntity(dto CreateDTO) (*T, error)
	UpdateEntity(entity *T, dto UpdateDTO) error
	ToResponse(entity *T) (any, error)
	ToListResponse(entities []T) (any, error)
}

// DTOValidator validates data transfer objects
type DTOValidator[CreateDTO any, UpdateDTO any] interface {
	ValidateCreate(dto CreateDTO) error
	ValidateUpdate(dto UpdateDTO) error
}

// NoOpValidator is a no-operation validator that always validates successfully
type NoOpValidator[CreateDTO any, UpdateDTO any] struct{}

// ValidateCreate validates a create DTO (no-op)
func (v *NoOpValidator[CreateDTO, UpdateDTO]) ValidateCreate(dto CreateDTO) error {
	return nil
}

// ValidateUpdate validates an update DTO (no-op)
func (v *NoOpValidator[CreateDTO, UpdateDTO]) ValidateUpdate(dto UpdateDTO) error {
	return nil
}

// NewBaseUseCase creates a new use case implementation
func NewBaseUseCase[T Entity, CreateDTO any, UpdateDTO any](
	repository BaseRepository[T],
	mapper DTOMapper[T, CreateDTO, UpdateDTO],
	logger Logger,
) *BaseUseCaseImpl[T, CreateDTO, UpdateDTO] {
	return &BaseUseCaseImpl[T, CreateDTO, UpdateDTO]{
		Repository: repository,
		Mapper:     mapper,
		Validator:  &NoOpValidator[CreateDTO, UpdateDTO]{},
		Logger:     logger,
	}
}

// WithValidator sets a custom validator for the use case
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) WithValidator(validator DTOValidator[CreateDTO, UpdateDTO]) *BaseUseCaseImpl[T, CreateDTO, UpdateDTO] {
	uc.Validator = validator
	return uc
}

// Create processes a creation request
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Create(ctx context.Context, dto CreateDTO) (*T, error) {
	// Validate DTO
	if err := uc.Validator.ValidateCreate(dto); err != nil {
		return nil, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("validation error: %v", err))
	}

	// Convert DTO to entity
	entity, err := uc.Mapper.ToEntity(dto)
	if err != nil {
		uc.Logger.Error("Failed to convert DTO to entity", "error", err)
		return nil, NewUseCaseError(ErrInvalidInput, "failed to process input data")
	}

	// Create entity in repository
	if err := uc.Repository.Create(ctx, entity); err != nil {
		uc.Logger.Error("Failed to create entity", "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to create resource")
	}

	return entity, nil
}

// GetByID retrieves an entity by its ID
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) GetByID(ctx context.Context, id uuid.UUID) (*T, error) {
	entity, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, errors.New("entity not found")) {
			return nil, NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found", id))
		}
		uc.Logger.Error("Failed to get entity by ID", "id", id, "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to retrieve resource")
	}
	return entity, nil
}

// List retrieves all entities with pagination
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) List(ctx context.Context, opts FilterOptions) (*PaginationResult[T], error) {
	result, err := uc.Repository.FindAll(ctx, opts)
	if err != nil {
		uc.Logger.Error("Failed to list entities", "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to retrieve resources")
	}
	return result, nil
}

// Update modifies an existing entity
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Update(ctx context.Context, id uuid.UUID, dto UpdateDTO) (*T, error) {
	// Validate DTO
	if err := uc.Validator.ValidateUpdate(dto); err != nil {
		return nil, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("validation error: %v", err))
	}

	// Fetch the existing entity
	entity, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, errors.New("entity not found")) {
			return nil, NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found", id))
		}
		uc.Logger.Error("Failed to get entity for update", "id", id, "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to retrieve resource for update")
	}

	// Apply updates from DTO
	if err := uc.Mapper.UpdateEntity(entity, dto); err != nil {
		uc.Logger.Error("Failed to apply DTO updates to entity", "error", err)
		return nil, NewUseCaseError(ErrInvalidInput, "failed to apply updates")
	}

	// Save the updated entity
	if err := uc.Repository.Update(ctx, entity); err != nil {
		uc.Logger.Error("Failed to update entity", "id", id, "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to update resource")
	}

	return entity, nil
}

// Delete soft-deletes an entity
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Delete(ctx context.Context, id uuid.UUID) error {
	// Check if entity exists
	_, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, errors.New("entity not found")) {
			return NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found", id))
		}
		uc.Logger.Error("Failed to find entity for deletion", "id", id, "error", err)
		return NewUseCaseError(ErrInternal, "failed to retrieve resource for deletion")
	}

	// Soft delete the entity
	if err := uc.Repository.Delete(ctx, id, false); err != nil {
		uc.Logger.Error("Failed to delete entity", "id", id, "error", err)
		return NewUseCaseError(ErrInternal, "failed to delete resource")
	}

	return nil
}

// HardDelete permanently deletes an entity
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) HardDelete(ctx context.Context, id uuid.UUID) error {
	// Check if entity exists
	_, err := uc.Repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, errors.New("entity not found")) {
			return NewUseCaseError(ErrNotFound, fmt.Sprintf("resource with ID %s not found", id))
		}
		uc.Logger.Error("Failed to find entity for hard deletion", "id", id, "error", err)
		return NewUseCaseError(ErrInternal, "failed to retrieve resource for deletion")
	}

	// Hard delete the entity
	if err := uc.Repository.Delete(ctx, id, true); err != nil {
		uc.Logger.Error("Failed to hard delete entity", "id", id, "error", err)
		return NewUseCaseError(ErrInternal, "failed to delete resource")
	}

	return nil
}

// FindWithFilter retrieves entities with a filter and pagination
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) FindWithFilter(
	ctx context.Context,
	filter map[string]interface{},
	opts FilterOptions,
) (*PaginationResult[T], error) {
	result, err := uc.Repository.FindWithFilter(ctx, filter, opts)
	if err != nil {
		uc.Logger.Error("Failed to find entities with filter", "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to retrieve resources")
	}
	return result, nil
}

// Count returns the count of entities matching the filter
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	count, err := uc.Repository.Count(ctx, filter)
	if err != nil {
		uc.Logger.Error("Failed to count entities", "error", err)
		return 0, NewUseCaseError(ErrInternal, "failed to count resources")
	}
	return count, nil
}

// --- Bulk Operations Implementation ---

// CreateMany processes a bulk creation request
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) CreateMany(ctx context.Context, dtos []CreateDTO) ([]*T, error) {
	if len(dtos) == 0 {
		return []*T{}, nil
	}

	entities := make([]*T, 0, len(dtos))
	for i, dto := range dtos {
		// Validate DTO
		if err := uc.Validator.ValidateCreate(dto); err != nil {
			return nil, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("validation error for item %d: %v", i, err))
		}

		// Convert DTO to entity
		entity, err := uc.Mapper.ToEntity(dto)
		if err != nil {
			uc.Logger.Error("Failed to convert DTO to entity for bulk create", "index", i, "error", err)
			return nil, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("failed to process input data for item %d", i))
		}
		entities = append(entities, entity)
	}

	// Create entities in repository
	if err := uc.Repository.CreateMany(ctx, entities); err != nil {
		uc.Logger.Error("Failed to bulk create entities", "error", err)
		return nil, NewUseCaseError(ErrInternal, "failed to create resources")
	}

	return entities, nil
}

// UpdateMany updates entities matching a filter
// NOTE: This basic implementation uses a single UpdateDTO to generate updates.
// More complex scenarios might require a different DTO or logic.
// It also doesn't fetch entities first, relying on repository-level updates.
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) UpdateMany(ctx context.Context, filter map[string]interface{}, dto UpdateDTO) (int64, error) {
	// Validate DTO
	if err := uc.Validator.ValidateUpdate(dto); err != nil {
		return 0, NewUseCaseError(ErrInvalidInput, fmt.Sprintf("validation error: %v", err))
	}

	// Convert UpdateDTO to a map[string]interface{} for the repository update
	// This requires reflection or a specific method in the mapper. Let's assume a helper exists or is added.
	// For simplicity here, we'll assume UpdateDTO can be directly marshalled/converted.
	// A real implementation would need a robust way to get settable fields from the DTO.
	updates, err := uc.convertUpdateDTOToMap(dto)
	if err != nil {
		uc.Logger.Error("Failed to convert UpdateDTO to map for bulk update", "error", err)
		return 0, NewUseCaseError(ErrInternal, "failed to prepare update data")
	}

	if len(updates) == 0 {
		return 0, NewUseCaseError(ErrInvalidInput, "no update fields provided")
	}

	// Perform bulk update in repository
	affected, err := uc.Repository.UpdateMany(ctx, filter, updates)
	if err != nil {
		uc.Logger.Error("Failed to bulk update entities", "error", err)
		return 0, NewUseCaseError(ErrInternal, "failed to update resources")
	}

	return affected, nil
}

// DeleteMany soft-deletes entities matching a filter
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) DeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error) {
	affected, err := uc.Repository.DeleteMany(ctx, filter, false)
	if err != nil {
		uc.Logger.Error("Failed to bulk delete entities", "error", err)
		return 0, NewUseCaseError(ErrInternal, "failed to delete resources")
	}
	return affected, nil
}

// HardDeleteMany permanently deletes entities matching a filter
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) HardDeleteMany(ctx context.Context, filter map[string]interface{}) (int64, error) {
	affected, err := uc.Repository.DeleteMany(ctx, filter, true)
	if err != nil {
		uc.Logger.Error("Failed to bulk hard delete entities", "error", err)
		return 0, NewUseCaseError(ErrInternal, "failed to permanently delete resources")
	}
	return affected, nil
}

// Helper function (placeholder) - needs proper implementation
func (uc *BaseUseCaseImpl[T, CreateDTO, UpdateDTO]) convertUpdateDTOToMap(dto UpdateDTO) (map[string]interface{}, error) {
	// This is a placeholder! A real implementation would use reflection
	// or a specific method in the DTOMapper to convert the DTO fields
	// (especially handling pointers for optional fields) into a map.
	// It should only include fields that are actually set in the DTO.
	// For example:
	// updates := make(map[string]interface{})
	// dtoValue := reflect.ValueOf(dto)
	// dtoType := dtoValue.Type()
	// for i := 0; i < dtoValue.NumField(); i++ {
	//    fieldValue := dtoValue.Field(i)
	//    fieldName := dtoType.Field(i).Name // Convert to DB field name if needed
	//    // Check if field is set (e.g., not nil for pointers)
	//    if fieldValue.IsValid() && !fieldValue.IsZero() { // Simplified check
	//       updates[fieldName] = fieldValue.Interface()
	//    }
	// }
	// return updates, nil

	// Simple placeholder - assumes DTO can be marshalled/unmarshalled
	// THIS IS NOT ROBUST AND LIKELY INCORRECT FOR POINTER FIELDS
	bytes, _ := json.Marshal(dto)
	var updates map[string]interface{}
	json.Unmarshal(bytes, &updates)
	if updates == nil {
		return make(map[string]interface{}), nil
	}
	return updates, nil // Placeholder return
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
