package internal

import (
	"context"

	"golang-microservices-boilerplate/pkg/core"

	"gorm.io/gorm"
)

// UserRepository interface defines data access methods for users
type UserRepository interface {
	core.BaseRepository[User]
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
}

// GormUserRepository implements UserRepository using GORM
type GormUserRepository struct {
	*core.GormBaseRepository[User]
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &GormUserRepository{
		GormBaseRepository: core.NewGormBaseRepository[User](db),
	}
}

// FindByEmail finds a user by their email address
func (r *GormUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	return r.FindOneWithFilter(ctx, map[string]interface{}{"email": email})
}

// FindByUsername finds a user by their username
func (r *GormUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	return r.FindOneWithFilter(ctx, map[string]interface{}{"username": username})
}
