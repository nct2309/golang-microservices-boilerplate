package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golang-microservices-boilerplate/pkg/core/entity"
	"golang-microservices-boilerplate/pkg/core/types"
)

// BaseRepository defines common database operations for all repositories
// Operates on pointers to entities (*T) where T implements entity.Entity
type BaseRepository[T entity.Entity] interface {
	Create(ctx context.Context, entity *T) error
	FindByID(ctx context.Context, id uuid.UUID) (*T, error)
	FindAll(ctx context.Context, opts types.FilterOptions) (*types.PaginationResult[T], error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
	FindWithFilter(ctx context.Context, filter map[string]interface{}, opts types.FilterOptions) (*types.PaginationResult[T], error)
	FindOneWithFilter(ctx context.Context, filter map[string]interface{}) (*T, error)
	Count(ctx context.Context, filter map[string]interface{}) (int64, error)
	Transaction(ctx context.Context, fn func(txRepo BaseRepository[T]) error) error

	// Bulk Operations
	CreateMany(ctx context.Context, entities []*T) ([]*T, error)
	UpdateMany(ctx context.Context, entities []*T) ([]*T, error)
	DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error
}

// GormBaseRepository implements the BaseRepository interface using GORM
// Reverted type parameters
type GormBaseRepository[T entity.Entity] struct {
	DB        *gorm.DB
	ModelType reflect.Type
}

// NewGormBaseRepository creates a new GORM-based repository
// Reverted type parameters
func NewGormBaseRepository[T entity.Entity](db *gorm.DB) *GormBaseRepository[T] {
	var modelPtr *T // Use pointer to get type
	ptrType := reflect.TypeOf(modelPtr)
	modelType := ptrType.Elem() // Get element type (the struct)
	return &GormBaseRepository[T]{
		DB:        db,
		ModelType: modelType,
	}
}

// Create adds a new entity to the database
func (r *GormBaseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.DB.WithContext(ctx).Create(entity).Error
}

// FindByID retrieves an entity by its ID
func (r *GormBaseRepository[T]) FindByID(ctx context.Context, id uuid.UUID) (*T, error) {
	entityPtr := reflect.New(r.ModelType).Interface().(*T)
	result := r.DB.WithContext(ctx).Where("id = ?", id).First(entityPtr)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("entity not found")
		}
		return nil, result.Error
	}
	return entityPtr, nil
}

// applyFilterOptions applies the provided filter options to a GORM query
func (r *GormBaseRepository[T]) applyFilterOptions(db *gorm.DB, opts types.FilterOptions) *gorm.DB {
	if len(opts.Filters) > 0 {
		db = db.Where(opts.Filters)
	}

	sortDirection := "ASC"
	if opts.SortDesc {
		sortDirection = "DESC"
	}
	if opts.SortBy != "" {
		db = db.Order(fmt.Sprintf("%s %s", opts.SortBy, sortDirection))
	}

	// Apply Limit and Offset
	if opts.Limit > 0 {
		db = db.Limit(opts.Limit)
	}
	if opts.Offset >= 0 { // Allow offset 0
		db = db.Offset(opts.Offset)
	}

	return db
}

// FindAll retrieves all entities of type *T with filter options
// Returns PaginationResult[T], Items field will hold []*T
func (r *GormBaseRepository[T]) FindAll(ctx context.Context, opts types.FilterOptions) (*types.PaginationResult[T], error) {
	var entities []*T // Slice of pointers
	var totalCount int64

	modelInstance := reflect.New(r.ModelType).Interface()
	db := r.DB.WithContext(ctx).Model(modelInstance)

	if !opts.IncludeDeleted {
		db = db.Where("deleted_at IS NULL")
	}

	// Apply filters/search for counting total items (without pagination)
	countDB := r.DB.WithContext(ctx).Model(modelInstance)
	if !opts.IncludeDeleted {
		countDB = countDB.Where("deleted_at IS NULL")
	}
	countOpts := types.FilterOptions{
		Filters:        opts.Filters,
		IncludeDeleted: opts.IncludeDeleted,
	}
	countDB = r.applyFilterOptions(countDB, countOpts)
	if err := countDB.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count items: %w", err)
	}

	// Apply all options for fetching items
	queryDB := r.applyFilterOptions(db, opts)
	if err := queryDB.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to find items: %w", err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	return &types.PaginationResult[T]{
		Items:      entities, // GORM Find populates []*T
		TotalItems: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// FindWithFilter retrieves entities that match the provided filter criteria
func (r *GormBaseRepository[T]) FindWithFilter(ctx context.Context, filter map[string]interface{}, opts types.FilterOptions) (*types.PaginationResult[T], error) {
	if opts.Filters == nil {
		opts.Filters = make(map[string]interface{})
	}
	for k, v := range filter {
		opts.Filters[k] = v
	}
	return r.FindAll(ctx, opts)
}

// Update modifies an existing entity
func (r *GormBaseRepository[T]) Update(ctx context.Context, entity *T) error {
	id := (*entity).GetID()
	if id == uuid.Nil {
		return errors.New("entity must have a valid ID for update")
	}
	return r.DB.WithContext(ctx).Model(entity).Where("id = ?", id).Updates(entity).Error
}

// FindOneWithFilter retrieves the first entity that matches the provided filter criteria
func (r *GormBaseRepository[T]) FindOneWithFilter(ctx context.Context, filter map[string]interface{}) (*T, error) {
	entityPtr := reflect.New(r.ModelType).Interface().(*T)
	db := r.DB.WithContext(ctx).Model(reflect.New(r.ModelType).Interface())

	if len(filter) > 0 {
		db = db.Where(filter)
	}
	db = db.Where("deleted_at IS NULL")

	result := db.First(entityPtr)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("entity not found")
		}
		return nil, result.Error
	}
	return entityPtr, nil
}

// Delete removes an entity from the database by ID
func (r *GormBaseRepository[T]) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	entityInstance := reflect.New(r.ModelType).Interface()
	db := r.DB.WithContext(ctx).Where("id = ?", id)

	var result *gorm.DB
	if hardDelete {
		result = db.Unscoped().Delete(entityInstance)
	} else {
		result = db.Delete(entityInstance)
	}
	return result.Error
}

// Count returns the count of entities matching the filter
func (r *GormBaseRepository[T]) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	var count int64
	modelInstance := reflect.New(r.ModelType).Interface()
	db := r.DB.WithContext(ctx).Model(modelInstance)

	if len(filter) > 0 {
		db = db.Where(filter)
	}
	db = db.Where("deleted_at IS NULL")

	err := db.Count(&count).Error
	return count, err
}

// Transaction runs a function within a database transaction
func (r *GormBaseRepository[T]) Transaction(ctx context.Context, fn func(txRepo BaseRepository[T]) error) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := &GormBaseRepository[T]{
			DB:        tx,
			ModelType: r.ModelType,
		}
		return fn(txRepo)
	})
}

// --- Bulk Operations Implementation ---

// CreateMany adds multiple entities to the database in a single batch.
// Returns the slice of created entities with DB-generated fields populated.
func (r *GormBaseRepository[T]) CreateMany(ctx context.Context, entities []*T) ([]*T, error) {
	if len(entities) == 0 {
		return entities, nil // Return empty slice, no error
	}
	err := r.DB.WithContext(ctx).Create(entities).Error
	if err != nil {
		return nil, err // Return nil slice on error
	}
	return entities, nil // Return the input slice, now populated by GORM
}

// UpdateMany updates multiple entities within a transaction based on the non-zero fields in the input entities.
// It then fetches and returns the full entities from the database after the update.
func (r *GormBaseRepository[T]) UpdateMany(ctx context.Context, entities []*T) ([]*T, error) {
	if len(entities) == 0 {
		return entities, nil // Return empty slice, no error
	}

	updatedIDs := make([]uuid.UUID, 0, len(entities))

	// Perform updates within a transaction
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			id := (*entity).GetID()
			if id == uuid.Nil {
				return fmt.Errorf("entity in bulk update list missing ID")
			}
			// Perform partial update based on the fields present in the input entity
			// Note: GORM's Updates only updates non-zero fields by default for structs.
			// If you need to update specific fields to zero values, use map[string]interface{} or Select.
			if err := tx.Model(entity).Where("id = ?", id).Updates(entity).Error; err != nil {
				return fmt.Errorf("failed to update entity with ID %s during bulk update: %w", id, err)
			}
			updatedIDs = append(updatedIDs, id) // Collect ID for re-fetching
		}
		return nil
	})

	if err != nil {
		return nil, err // Return nil slice on transaction error
	}

	// If updates were successful, fetch the full entities
	if len(updatedIDs) > 0 {
		var updatedEntities []*T
		if err := r.DB.WithContext(ctx).Where("id IN (?)", updatedIDs).Find(&updatedEntities).Error; err != nil {
			// Log the error, but perhaps still return the original entities or handle differently?
			// Returning an error here might be confusing if the update itself succeeded.
			// For now, let's return the fetch error.
			return nil, fmt.Errorf("updates succeeded, but failed to fetch updated entities: %w", err)
		}
		return updatedEntities, nil
	}

	// Should not happen if input entities slice was not empty, but return empty slice just in case.
	return []*T{}, nil
}

// DeleteMany removes multiple entities matching the provided IDs.
func (r *GormBaseRepository[T]) DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error {
	if len(ids) == 0 {
		return nil
	}

	modelInstance := reflect.New(r.ModelType).Interface()
	db := r.DB.WithContext(ctx).Where("id IN (?)", ids)

	var result *gorm.DB
	if hardDelete {
		result = db.Unscoped().Delete(modelInstance)
	} else {
		result = db.Delete(modelInstance)
	}

	if result.Error != nil {
		return fmt.Errorf("failed during bulk delete: %w", result.Error)
	}

	return nil
}
