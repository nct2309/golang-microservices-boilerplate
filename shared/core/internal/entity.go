package internal

import (
	"time"

	"github.com/google/uuid"
)

// BaseEntity struct to be embedded in other structs
type BaseEntity struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;default:uuid_generate_v4()"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// BeforeCreate hook to set the ID before creating a new record
func (base *BaseEntity) BeforeCreate() (err error) {
	base.ID = uuid.New()
	return
}
