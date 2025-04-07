package core

import (
	"context"
	"errors"
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FilterOptions provides common filtering, pagination and sorting options
type FilterOptions struct {
	Limit          int                    `json:"limit"`
	Offset         int                    `json:"offset"`
	SortBy         string                 `json:"sort_by"`
	SortDesc       bool                   `json:"sort_desc"`
	Filters        map[string]interface{} `json:"filters"`
	SearchTerm     string                 `json:"search_term"`
	SearchFields   []string               `json:"search_fields"`
	IncludeDeleted bool                   `json:"include_deleted"`
}

// DefaultFilterOptions returns a default set of filter options
func DefaultFilterOptions() FilterOptions {
	return FilterOptions{
		Limit:    50,
		Offset:   0,
		SortBy:   "created_at",
		SortDesc: true,
		Filters:  make(map[string]interface{}),
	}
}

// PaginationResult contains the results of a paginated query
type PaginationResult[T any] struct {
	Data       []T   `json:"data"`
	TotalCount int64 `json:"total_count"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// BaseRepository defines common database operations for all repositories
type BaseRepository[T Entity] interface {
	Create(ctx context.Context, entity *T) error
	FindByID(ctx context.Context, id uuid.UUID) (*T, error)
	FindAll(ctx context.Context, opts FilterOptions) (*PaginationResult[T], error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
	FindWithFilter(ctx context.Context, filter map[string]interface{}, opts FilterOptions) (*PaginationResult[T], error)
	FindOneWithFilter(ctx context.Context, filter map[string]interface{}) (*T, error)
	Count(ctx context.Context, filter map[string]interface{}) (int64, error)
	Transaction(ctx context.Context, fn func(txRepo BaseRepository[T]) error) error

	// Bulk Operations
	CreateMany(ctx context.Context, entities []*T) error
	UpdateMany(ctx context.Context, filter map[string]interface{}, updates map[string]interface{}) (int64, error) // Returns number of affected rows
	DeleteMany(ctx context.Context, filter map[string]interface{}, hardDelete bool) (int64, error)                // Returns number of affected rows
}

// GormBaseRepository implements the BaseRepository interface using GORM
type GormBaseRepository[T Entity] struct {
	DB        *gorm.DB
	ModelType reflect.Type
}

// NewGormBaseRepository creates a new GORM-based repository
func NewGormBaseRepository[T Entity](db *gorm.DB) *GormBaseRepository[T] {
	var modelType T
	typeOf := reflect.TypeOf(modelType)

	// If T is already a non-pointer concrete type
	if typeOf.Kind() != reflect.Pointer {
		return &GormBaseRepository[T]{
			DB:        db,
			ModelType: typeOf,
		}
	}

	// If T is a pointer type, get the element type
	return &GormBaseRepository[T]{
		DB:        db,
		ModelType: typeOf.Elem(),
	}
}

// Create adds a new entity to the database
func (r *GormBaseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.DB.WithContext(ctx).Create(entity).Error
}

// FindByID retrieves an entity by its ID
func (r *GormBaseRepository[T]) FindByID(ctx context.Context, id uuid.UUID) (*T, error) {
	var entity T
	result := r.DB.WithContext(ctx).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("entity not found")
		}
		return nil, result.Error
	}
	return &entity, nil
}

// applyFilterOptions applies the provided filter options to a GORM query
func (r *GormBaseRepository[T]) applyFilterOptions(db *gorm.DB, opts FilterOptions) *gorm.DB {
	// Apply filters
	if len(opts.Filters) > 0 {
		db = db.Where(opts.Filters)
	}

	// Apply search term if provided
	if opts.SearchTerm != "" && len(opts.SearchFields) > 0 {
		searchQuery := db
		for i, field := range opts.SearchFields {
			if i == 0 {
				searchQuery = searchQuery.Where(field+" LIKE ?", "%"+opts.SearchTerm+"%")
			} else {
				searchQuery = searchQuery.Or(field+" LIKE ?", "%"+opts.SearchTerm+"%")
			}
		}
		db = db.Where(searchQuery)
	}

	// Apply sorting
	sortDirection := "ASC"
	if opts.SortDesc {
		sortDirection = "DESC"
	}
	if opts.SortBy != "" {
		db = db.Order(opts.SortBy + " " + sortDirection)
	}

	// Apply pagination
	if opts.Limit > 0 {
		db = db.Limit(opts.Limit)
	}
	if opts.Offset >= 0 {
		db = db.Offset(opts.Offset)
	}

	return db
}

// FindAll retrieves all entities of type T with filter options
func (r *GormBaseRepository[T]) FindAll(ctx context.Context, opts FilterOptions) (*PaginationResult[T], error) {
	var entities []T
	var totalCount int64

	db := r.DB.WithContext(ctx).Model(reflect.New(r.ModelType).Interface())

	// If we shouldn't include soft-deleted items, add the clause
	if !opts.IncludeDeleted {
		db = db.Where("deleted_at IS NULL")
	}

	// Count total records (before pagination)
	countDB := r.applyFilterOptions(db.Session(&gorm.Session{}), FilterOptions{
		Filters:        opts.Filters,
		SearchTerm:     opts.SearchTerm,
		SearchFields:   opts.SearchFields,
		IncludeDeleted: opts.IncludeDeleted,
	})
	if err := countDB.Count(&totalCount).Error; err != nil {
		return nil, err
	}

	// Apply all filter options for data retrieval
	queryDB := r.applyFilterOptions(db, opts)
	if err := queryDB.Find(&entities).Error; err != nil {
		return nil, err
	}

	// Calculate pagination metadata
	pageSize := opts.Limit
	if pageSize <= 0 {
		pageSize = len(entities)
	}

	currentPage := 0
	if pageSize > 0 {
		currentPage = opts.Offset / pageSize
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	}

	return &PaginationResult[T]{
		Data:       entities,
		TotalCount: totalCount,
		Page:       currentPage,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// FindWithFilter retrieves entities that match the provided filter criteria with pagination support
func (r *GormBaseRepository[T]) FindWithFilter(ctx context.Context, filter map[string]interface{}, opts FilterOptions) (*PaginationResult[T], error) {
	// Merge the provided filter with the filter in options
	// if filter != nil {
	for k, v := range filter {
		opts.Filters[k] = v
	}
	// }
	return r.FindAll(ctx, opts)
}

// Update modifies an existing entity
func (r *GormBaseRepository[T]) Update(ctx context.Context, entity *T) error {
	// Convert to interface to call GetID method
	entityInterface := any(*entity)
	entityWithID, ok := entityInterface.(Entity)
	if !ok {
		return errors.New("entity must implement Entity interface")
	}

	id := entityWithID.GetID()
	if id == uuid.Nil {
		return errors.New("entity must have a valid ID for update")
	}

	return r.DB.WithContext(ctx).Where("id = ?", id).Updates(entity).Error
}

// FindOneWithFilter retrieves the first entity that matches the provided filter criteria
func (r *GormBaseRepository[T]) FindOneWithFilter(ctx context.Context, filter map[string]interface{}) (*T, error) {
	var entity T
	result := r.DB.WithContext(ctx).Where(filter).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("entity not found")
		}
		return nil, result.Error
	}
	return &entity, nil
}

// Delete removes an entity from the database by ID
func (r *GormBaseRepository[T]) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	var entity T
	db := r.DB.WithContext(ctx).Where("id = ?", id)

	if hardDelete {
		// For hard delete
		result := db.Unscoped().Delete(&entity)
		return result.Error
	} else {
		// For soft delete
		result := db.Delete(&entity)
		return result.Error
	}
}

// Count returns the count of entities matching the filter
func (r *GormBaseRepository[T]) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	var count int64
	db := r.DB.WithContext(ctx).Model(reflect.New(r.ModelType).Interface())

	if filter != nil {
		db = db.Where(filter)
	}

	err := db.Count(&count).Error
	return count, err
}

// Transaction runs a function within a database transaction
func (r *GormBaseRepository[T]) Transaction(ctx context.Context, fn func(txRepo BaseRepository[T]) error) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := NewGormBaseRepository[T](tx)
		return fn(txRepo)
	})
}

// --- Bulk Operations Implementation ---

// CreateMany adds multiple entities to the database in a single batch
func (r *GormBaseRepository[T]) CreateMany(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil // Nothing to create
	}
	// Note: GORM's Create in batches handles hooks if called correctly.
	// Using CreateInBatches might be more efficient for very large lists.
	return r.DB.WithContext(ctx).Create(entities).Error
}

// UpdateMany updates multiple entities matching the filter with the provided updates
// Returns the number of affected rows.
func (r *GormBaseRepository[T]) UpdateMany(ctx context.Context, filter map[string]interface{}, updates map[string]interface{}) (int64, error) {
	if len(filter) == 0 {
		return 0, errors.New("filter cannot be empty for UpdateMany")
	}
	if len(updates) == 0 {
		return 0, errors.New("updates cannot be empty for UpdateMany")
	}

	db := r.DB.WithContext(ctx).Model(reflect.New(r.ModelType).Interface()).Where(filter)
	result := db.Updates(updates)
	return result.RowsAffected, result.Error
}

// DeleteMany removes multiple entities matching the filter
// Returns the number of affected rows.
func (r *GormBaseRepository[T]) DeleteMany(ctx context.Context, filter map[string]interface{}, hardDelete bool) (int64, error) {
	if len(filter) == 0 {
		return 0, errors.New("filter cannot be empty for DeleteMany")
	}

	db := r.DB.WithContext(ctx).Where(filter)

	// Create a zero value instance of the model type for GORM's Delete
	// This is necessary for GORM to know which table to operate on
	modelInstance := reflect.New(r.ModelType).Interface()

	var result *gorm.DB
	if hardDelete {
		// For hard delete
		result = db.Unscoped().Delete(modelInstance)
	} else {
		// For soft delete
		result = db.Delete(modelInstance)
	}
	return result.RowsAffected, result.Error
}
