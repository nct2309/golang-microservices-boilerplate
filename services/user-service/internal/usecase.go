package internal

import (
	"context"
	"time"

	"golang-microservices-boilerplate/pkg/core"
	"golang-microservices-boilerplate/pkg/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
)

// UserUseCase defines the business logic for user operations
type UserUseCase interface {
	core.BaseUseCase[User, CreateUserDTO, UpdateUserDTO]
	Login(ctx context.Context, email, password string) (*User, string, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

// UserUseCaseImpl implements UserUseCase
type UserUseCaseImpl struct {
	*core.BaseUseCaseImpl[User, CreateUserDTO, UpdateUserDTO]
	repo      UserRepository
	jwtSecret string
	jwtExpiry time.Duration
	logger    core.Logger
}

// UserDTOMapper implements the core.DTOMapper interface for User entities
type UserDTOMapper struct{}

// ToEntity converts a CreateUserDTO to a User entity
func (m *UserDTOMapper) ToEntity(dto CreateUserDTO) (*User, error) {
	// Create a new user entity
	user := &User{
		Email:     dto.Email,
		Username:  dto.Username,
		Password:  dto.Password, // Will be hashed in BeforeCreate hook
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
	}

	if dto.Role != "" {
		user.Role = Role(dto.Role)
	}

	return user, nil
}

// UpdateEntity updates a User entity with values from UpdateUserDTO
func (m *UserDTOMapper) UpdateEntity(entity *User, dto UpdateUserDTO) error {
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

// ToResponse implements the DTOMapper interface (even though we don't use it directly)
func (m *UserDTOMapper) ToResponse(entity *User) (interface{}, error) {
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

// ToListResponse implements the DTOMapper interface (even though we don't use it directly)
func (m *UserDTOMapper) ToListResponse(entities []User) (interface{}, error) {
	result := make([]UserResponseDTO, 0, len(entities))

	for _, entity := range entities {
		resp, err := m.ToResponse(&entity)
		if err != nil {
			return nil, err
		}
		result = append(result, resp.(UserResponseDTO))
	}

	return result, nil
}

// NewUserUseCase creates a new UserUseCase
func NewUserUseCase(repo UserRepository, logger core.Logger) UserUseCase {
	mapper := &UserDTOMapper{}
	validator := &UserValidator{}

	baseUseCase := core.NewBaseUseCase(
		repo,
		mapper,
		logger,
	).WithValidator(validator)

	return &UserUseCaseImpl{
		BaseUseCaseImpl: baseUseCase,
		repo:            repo,
		jwtSecret:       utils.GetEnv("JWT_SECRET", "your-secret-key"),
		jwtExpiry:       time.Hour * 24,
		logger:          logger,
	}
}

// Login authenticates a user with email and password
func (uc *UserUseCaseImpl) Login(ctx context.Context, email, password string) (*User, string, error) {
	user, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, "", core.NewUseCaseError(core.ErrNotFound, "user not found")
	}

	if !user.CheckPassword(password) {
		return nil, "", core.NewUseCaseError(core.ErrUnauthorized, "invalid credentials")
	}

	if !user.IsActive {
		return nil, "", core.NewUseCaseError(core.ErrUnauthorized, "account is inactive")
	}

	// Update last login time
	user.UpdateLoginTime()
	if err := uc.repo.Update(ctx, user); err != nil {
		uc.logger.Error("Failed to update last login time", "error", err)
		// Don't fail the login for this non-critical update
	}

	// Generate JWT token
	token, err := GenerateJWT(map[string]interface{}{
		"user_id": user.ID.String(),
		"role":    string(user.Role),
		"email":   user.Email,
	}, uc.jwtSecret, uc.jwtExpiry)

	if err != nil {
		uc.logger.Error("Failed to generate JWT", "error", err)
		return nil, "", core.NewUseCaseError(core.ErrInternal, "error generating authentication token")
	}

	return user, token, nil
}

// FindByEmail finds a user by their email address
func (uc *UserUseCaseImpl) FindByEmail(ctx context.Context, email string) (*User, error) {
	user, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, core.NewUseCaseError(core.ErrNotFound, "user not found")
	}
	return user, nil
}

// GenerateJWT generates a new JWT token with the given claims
func GenerateJWT(claims map[string]interface{}, secret string, expiry time.Duration) (string, error) {
	// Create token with claims
	tokenClaims := jwt.MapClaims{}

	// Add standard claims
	now := time.Now()
	expiryTime := now.Add(expiry)
	tokenClaims["iat"] = now.Unix()
	tokenClaims["exp"] = expiryTime.Unix()

	// Add custom claims
	for key, value := range claims {
		tokenClaims[key] = value
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)

	// Sign token with secret
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", errors.Wrap(err, "failed to sign JWT token")
	}

	return tokenString, nil
}
