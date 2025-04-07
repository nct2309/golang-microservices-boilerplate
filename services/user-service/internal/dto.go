package internal

import (
	"time"

	"golang-microservices-boilerplate/pkg/core"
)

// CreateUserDTO represents the data needed to create a new user
type CreateUserDTO struct {
	Username  string `json:"username"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role" validate:"omitempty,oneof=admin manager officer"`
}

// UpdateUserDTO represents the data that can be updated for a user
type UpdateUserDTO struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password" validate:"omitempty,min=8"`
	Role      string `json:"role" validate:"omitempty,oneof=admin manager officer"`
	IsActive  *bool  `json:"is_active"`
}

// UserResponseDTO represents a user in API responses
type UserResponseDTO struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Role        string     `json:"role"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// UserMapper implements DTOMapper for User entities
type UserMapper struct{}

// ToEntity converts a CreateUserDTO to a User entity
func (m *UserMapper) ToEntity(dto CreateUserDTO) (*User, error) {
	user := &User{
		Username:  dto.Username,
		Email:     dto.Email,
		Password:  dto.Password,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
	}

	if dto.Role != "" {
		user.Role = Role(dto.Role)
	}

	return user, nil
}

// UpdateEntity applies an UpdateUserDTO to a User entity
func (m *UserMapper) UpdateEntity(entity *User, dto UpdateUserDTO) error {
	if dto.FirstName != "" {
		entity.FirstName = dto.FirstName
	}

	if dto.LastName != "" {
		entity.LastName = dto.LastName
	}

	if dto.Password != "" {
		if err := entity.SetPassword(dto.Password); err != nil {
			return err
		}
	}

	if dto.Role != "" {
		entity.Role = Role(dto.Role)
	}

	if dto.IsActive != nil {
		entity.IsActive = *dto.IsActive
	}

	return nil
}

// ToResponse converts a User entity to a UserResponseDTO
func (m *UserMapper) ToResponse(entity *User) (any, error) {
	if entity == nil {
		return nil, nil
	}

	return UserResponseDTO{
		ID:          entity.ID.String(),
		Username:    entity.Username,
		Email:       entity.Email,
		FirstName:   entity.FirstName,
		LastName:    entity.LastName,
		Role:        string(entity.Role),
		IsActive:    entity.IsActive,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
		LastLoginAt: entity.LastLoginAt,
	}, nil
}

// ToListResponse converts a slice of User entities to a list response
func (m *UserMapper) ToListResponse(entities []User) (any, error) {
	users := make([]UserResponseDTO, 0, len(entities))

	for _, entity := range entities {
		resp, err := m.ToResponse(&entity)
		if err != nil {
			return nil, err
		}
		users = append(users, resp.(UserResponseDTO))
	}

	return users, nil
}

// UserValidator validates user DTOs
type UserValidator struct{}

// ValidateCreate validates a CreateUserDTO
func (v *UserValidator) ValidateCreate(dto CreateUserDTO) error {
	if dto.Email == "" {
		return core.NewUseCaseError(core.ErrInvalidInput, "email is required")
	}

	if dto.Password == "" {
		return core.NewUseCaseError(core.ErrInvalidInput, "password is required")
	}

	if len(dto.Password) < 8 {
		return core.NewUseCaseError(core.ErrInvalidInput, "password must be at least 8 characters")
	}

	return nil
}

// ValidateUpdate validates an UpdateUserDTO
func (v *UserValidator) ValidateUpdate(dto UpdateUserDTO) error {
	if dto.Password != "" && len(dto.Password) < 8 {
		return core.NewUseCaseError(core.ErrInvalidInput, "password must be at least 8 characters")
	}

	if dto.Role != "" &&
		dto.Role != string(RoleAdmin) &&
		dto.Role != string(RoleManager) &&
		dto.Role != string(RoleOfficer) {
		return core.NewUseCaseError(core.ErrInvalidInput, "invalid role")
	}

	return nil
}
