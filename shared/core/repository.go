package core

import (
	"context"
	"errors"
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseRepository defines common database operations for all repositories
type BaseRepository[T any] interface {
	Create(ctx context.Context, entity *T) error
	FindByID(ctx context.Context, id uuid.UUID) (*T, error)
	FindAll(ctx context.Context) ([]T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindWithFilter(ctx context.Context, filter map[string]interface{}) ([]T, error)
	FindOneWithFilter(ctx context.Context, filter map[string]interface{}) (*T, error)
}

// GormBaseRepository implements the BaseRepository interface using GORM
type GormBaseRepository[T any] struct {
	DB *gorm.DB
}

// NewGormBaseRepository creates a new GORM-based repository
func NewGormBaseRepository[T any](db *gorm.DB) *GormBaseRepository[T] {
	return &GormBaseRepository[T]{
		DB: db,
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

// FindAll retrieves all entities of type T
func (r *GormBaseRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T
	result := r.DB.WithContext(ctx).Find(&entities)
	if result.Error != nil {
		return nil, result.Error
	}
	return entities, nil
}

// Update modifies an existing entity
func (r *GormBaseRepository[T]) Update(ctx context.Context, entity *T) error {
	// Get the ID field from the entity
	val := reflect.ValueOf(entity).Elem()
	idField := val.FieldByName("ID")

	if !idField.IsValid() || idField.IsZero() {
		return errors.New("entity must have a valid ID for update")
	}

	id := idField.Interface().(uuid.UUID)

	return r.DB.WithContext(ctx).Where("id = ?", id).Updates(entity).Error
}

// Delete removes an entity from the database by ID
func (r *GormBaseRepository[T]) Delete(ctx context.Context, id uuid.UUID) error {
	var entity T
	result := r.DB.WithContext(ctx).Where("id = ?", id).Delete(&entity)
	return result.Error
}

// FindWithFilter retrieves entities that match the provided filter criteria
func (r *GormBaseRepository[T]) FindWithFilter(ctx context.Context, filter map[string]interface{}) ([]T, error) {
	var entities []T
	result := r.DB.WithContext(ctx).Where(filter).Find(&entities)
	if result.Error != nil {
		return nil, result.Error
	}
	return entities, nil
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
