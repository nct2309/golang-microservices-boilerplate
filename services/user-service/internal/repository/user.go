package repository

import (
	"context"

	core_repo "golang-microservices-boilerplate/pkg/core/repository"
	"golang-microservices-boilerplate/services/user-service/internal/entity"

	"gorm.io/gorm"
)

// UserRepository defines the specific persistence operations for User entities,
// extending the generic BaseRepository functionality.
type UserRepository interface {
	// Embed the generic BaseRepository for common CRUD operations
	// Note: The core repository operates on POINTERS (*T), so methods like Create/Update expect *entity.User
	// and methods like FindByID/FindOne return *entity.User.
	core_repo.BaseRepository[entity.User] // T is entity.User, which implements entity.Entity

	// FindByEmail retrieves a user by their email address.
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}

// gormUserRepository implements UserRepository using GORM
// It embeds the generic GORM repository and provides specific finders.
type gormUserRepository struct {
	// Embed the generic GORM repository specialized for entity.User.
	// This provides implementations for the methods in core_repo.BaseRepository[entity.User].
	*core_repo.GormBaseRepository[entity.User]
}

// NewUserRepository creates a new UserRepository using the provided GORM DB connection.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &gormUserRepository{
		GormBaseRepository: core_repo.NewGormBaseRepository[entity.User](db),
	}
}

// --- Implement UserRepository Specific Methods ---

// FindByEmail finds a user by their email address using the embedded FindOneWithFilter.
func (r *gormUserRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	filter := map[string]interface{}{"email": email}
	// Use the embedded FindOneWithFilter method from GormBaseRepository.
	return r.FindOneWithFilter(ctx, filter)
}

/*
// Example implementation for FindByUsername
func (r *gormUserRepository) FindByUsername(ctx context.Context, username string) (*entity.User, error) {
	filter := map[string]interface{}{"username": username}
	return r.FindOneWithFilter(ctx, filter)
}
*/
