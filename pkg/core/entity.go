package core

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Entity defines the interface that all domain entities must implement
type Entity interface {
	GetID() uuid.UUID
	SetID(id uuid.UUID)
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

// BaseEntity struct to be embedded in other structs
type BaseEntity struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// GetID returns the entity ID
func (base *BaseEntity) GetID() uuid.UUID {
	return base.ID
}

// SetID sets the entity ID
func (base *BaseEntity) SetID(id uuid.UUID) {
	base.ID = id
}

// GetCreatedAt returns the creation timestamp
func (base *BaseEntity) GetCreatedAt() time.Time {
	return base.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (base *BaseEntity) GetUpdatedAt() time.Time {
	return base.UpdatedAt
}

// BeforeCreate hook to set the ID before creating a new record
func (base *BaseEntity) BeforeCreate(tx *gorm.DB) (err error) {
	if base.ID == uuid.Nil {
		base.ID = uuid.New()
	}
	return nil
}

// IsDeleted checks if the entity has been soft deleted
func (base *BaseEntity) IsDeleted() bool {
	return base.DeletedAt != nil
}

// Clone creates a copy of a BaseEntity for safe modification
func (base *BaseEntity) Clone() BaseEntity {
	return BaseEntity{
		ID:        base.ID,
		CreatedAt: base.CreatedAt,
		UpdatedAt: base.UpdatedAt,
		DeletedAt: base.DeletedAt,
	}
}
