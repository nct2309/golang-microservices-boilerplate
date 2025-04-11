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
	CreateMany(ctx context.Context, entities []*T) error
	UpdateMany(ctx context.Context, entities []*T) error
	DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error
}

// GormBaseRepository implements the BaseRepository interface using GORM
type GormBaseRepository[T entity.Entity] struct {
	DB        *gorm.DB
	ModelType reflect.Type
}

// NewGormBaseRepository creates a new GORM-based repository
func NewGormBaseRepository[T entity.Entity](db *gorm.DB) *GormBaseRepository[T] {
	var modelPtr *T
	ptrType := reflect.TypeOf(modelPtr)
	modelType := ptrType.Elem()
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
// It returns a pointer to PaginationResult[T], which internally holds []*T
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
		// Exclude limit/offset/sort for count
	}
	countDB = r.applyFilterOptions(countDB, countOpts)
	if err := countDB.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count items: %w", err)
	}

	// Apply all options (including limit/offset/sort) for fetching items
	queryDB := r.applyFilterOptions(db, opts)
	if err := queryDB.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to find items: %w", err)
	}

	// Ensure limit and offset reflect the actual query params used
	limit := opts.Limit
	if limit <= 0 {
		limit = 50 // Use default if invalid
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0 // Use default if invalid
	}

	// Construct and return a pointer to PaginationResult[T]
	return &types.PaginationResult[T]{
		Items:      entities, // entities is already []*T
		TotalItems: totalCount,
		Limit:      limit,  // Reflect the limit used
		Offset:     offset, // Reflect the offset used
	}, nil
}

// FindWithFilter retrieves entities that match the provided filter criteria with pagination support
func (r *GormBaseRepository[T]) FindWithFilter(ctx context.Context, filter map[string]interface{}, opts types.FilterOptions) (*types.PaginationResult[T], error) {
	if opts.Filters == nil {
		opts.Filters = make(map[string]interface{})
	}
	for k, v := range filter {
		opts.Filters[k] = v
	}
	// Delegates to FindAll, which correctly returns *types.PaginationResult[T]
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

// CreateMany adds multiple entities to the database in a single batch
func (r *GormBaseRepository[T]) CreateMany(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Create(entities).Error
}

// UpdateMany updates multiple entities within a transaction.
// It iterates through the provided entities and updates each one based on its ID.
func (r *GormBaseRepository[T]) UpdateMany(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil // Nothing to update
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			id := (*entity).GetID()
			if id == uuid.Nil {
				// Optionally log the entity index or skip
				return fmt.Errorf("entity in bulk update list missing ID")
			}
			// Use tx database handle for updates within the transaction
			if err := tx.Model(entity).Where("id = ?", id).Updates(entity).Error; err != nil {
				// Optionally include ID in the error message
				return fmt.Errorf("failed to update entity with ID %s during bulk update: %w", id, err)
			}
		}
		return nil
	})
}

// DeleteMany removes multiple entities matching the provided IDs.
func (r *GormBaseRepository[T]) DeleteMany(ctx context.Context, ids []uuid.UUID, hardDelete bool) error {
	if len(ids) == 0 {
		return nil // Nothing to delete
	}

	modelInstance := reflect.New(r.ModelType).Interface()
	db := r.DB.WithContext(ctx).Where("id IN (?)", ids)

	var result *gorm.DB
	if hardDelete {
		// Important: Use Unscoped for hard delete
		result = db.Unscoped().Delete(modelInstance)
	} else {
		result = db.Delete(modelInstance)
	}

	// Check for errors, RowsAffected can be checked if needed but error is primary
	if result.Error != nil {
		return fmt.Errorf("failed during bulk delete: %w", result.Error)
	}

	// Optional: Check if the number of affected rows matches len(ids)
	// if result.RowsAffected != int64(len(ids)) { ... log or return specific error ... }

	return nil
}
