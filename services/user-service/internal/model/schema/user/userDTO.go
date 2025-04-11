package schema

import (
	core_entity "golang-microservices-boilerplate/pkg/core/entity"
	"golang-microservices-boilerplate/services/user-service/internal/model/entity" // Assuming entity is in this path
	"time"
)

// UserCreateDTO holds the data for creating a new user.
// Fields are tagged for validation using conventions from pkg/core/dto/validation.go
type UserCreateDTO struct {
	Username  string      `json:"username" validate:"required,alphanum,min=3,max=30"`
	Email     string      `json:"email" validate:"required,email"`
	Password  string      `json:"password" validate:"required,min=8"`
	FirstName string      `json:"first_name,omitempty" validate:"omitempty,alpha,max=50"`
	LastName  string      `json:"last_name,omitempty" validate:"omitempty,alpha,max=50"`
	Role      entity.Role `json:"role,omitempty" validate:"omitempty,oneof=admin manager officer"` // Use Role type from entity

	// Optional fields
	Phone      *string `json:"phone,omitempty" validate:"omitempty,e164"`
	Address    *string `json:"address,omitempty" validate:"omitempty,max=255"`
	Age        *int    `json:"age,omitempty" validate:"omitempty,min=10,max=100"`
	ProfilePic *string `json:"profile_pic,omitempty" validate:"omitempty,url"`
}

// UserUpdateDTO holds the data for updating an existing user.
// All fields are optional (pointers) and validated if present.
type UserUpdateDTO struct {
	Username  string      `json:"username,omitempty" validate:"omitempty,alphanum,min=3,max=30"`
	Email     string      `json:"email,omitempty" validate:"omitempty,email"`
	Password  string      `json:"password,omitempty" validate:"omitempty,min=8"` // Allows password change
	FirstName string      `json:"first_name,omitempty" validate:"omitempty,alpha,max=50"`
	LastName  string      `json:"last_name,omitempty" validate:"omitempty,alpha,max=50"`
	Role      entity.Role `json:"role,omitempty" validate:"omitempty,oneof=admin manager officer"`
	IsActive  bool        `json:"is_active,omitempty"` // Allows activation/deactivation

	// Optional fields
	Phone      string `json:"phone,omitempty" validate:"omitempty,e164"`
	Address    string `json:"address,omitempty" validate:"omitempty,max=255"`
	Age        int    `json:"age,omitempty" validate:"omitempty,min=10,max=100"`
	ProfilePic string `json:"profile_pic,omitempty" validate:"omitempty,url"`
}

type UserResponseDTO struct {
	core_entity.BaseEntityDTO
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	FirstName   string      `json:"first_name"`
	LastName    string      `json:"last_name"`
	Role        entity.Role `json:"role"`
	IsActive    bool        `json:"is_active"`
	LastLoginAt *time.Time  `json:"last_login_at"`

	// Optional fields
	Phone      string `json:"phone,omitempty"`
	Address    string `json:"address,omitempty"`
	Age        int    `json:"age,omitempty"`
	ProfilePic string `json:"profile_pic,omitempty"`
}
