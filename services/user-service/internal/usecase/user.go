package usecase

import (
	"context"
	"time"

	core_entity "golang-microservices-boilerplate/pkg/core/entity"
	core_logger "golang-microservices-boilerplate/pkg/core/logger"
	core_usecase "golang-microservices-boilerplate/pkg/core/usecase"
	"golang-microservices-boilerplate/services/user-service/internal/model/entity"
	schema "golang-microservices-boilerplate/services/user-service/internal/model/schema/user"
	user_repository "golang-microservices-boilerplate/services/user-service/internal/repository"

	"github.com/google/uuid"
)

// Specific error messages to check against repository errors
const errUserNotFoundMsg = "entity not found"

// Define JWT expiration durations (can be configured externally)
const (
	defaultAccessTokenDuration  = 7 * 24 * time.Hour  // 7 days
	defaultRefreshTokenDuration = 30 * 24 * time.Hour // 30 days
)

// TokenGenerator defines the interface for generating JWT tokens.
// It now takes a map for claims directly.
type TokenGenerator interface {
	GenerateTokenPair(customClaims map[string]interface{}, accessDuration, refreshDuration time.Duration) (accessToken, refreshToken string, expiresAt int64, err error)
}

// LoginCredentials, LoginResult, RefreshResult are now defined in the schema package
// type LoginCredentials struct { ... }
// type LoginResult struct { ... }
// type RefreshResult struct { ... }

// UserUsecase defines the business logic operations for users.
// It now uses DTOs and result types from the schema package.
type UserUsecase interface {
	core_usecase.BaseUseCase[entity.User, schema.UserCreateDTO, schema.UserUpdateDTO]      // Use schema DTOs
	Login(ctx context.Context, creds schema.LoginCredentials) (*schema.LoginResult, error) // Use schema types
	Refresh(ctx context.Context, refreshToken string) (*schema.RefreshResult, error)       // Use schema type
}

// userUseCaseImpl implements the UserUsecase interface.
type userUseCaseImpl struct {
	*core_usecase.BaseUseCaseImpl[entity.User, schema.UserCreateDTO, schema.UserUpdateDTO] // Use schema DTOs
	userRepo                                                                               user_repository.UserRepository
	logger                                                                                 core_logger.Logger
	tokenGen                                                                               TokenGenerator
	accessTokenDuration                                                                    time.Duration
	refreshTokenDuration                                                                   time.Duration
}

// NewUserUseCase creates a new instance of UserUsecase.
func NewUserUseCase(
	userRepo user_repository.UserRepository,
	logger core_logger.Logger,
	tokenGen TokenGenerator,
	accessTokenDur *time.Duration,
	refreshTokenDur *time.Duration,
) UserUsecase { // Return the UserUsecase interface type
	// Use schema DTOs when creating the base use case
	baseUseCase := core_usecase.NewBaseUseCase[entity.User, schema.UserCreateDTO, schema.UserUpdateDTO](userRepo, logger)
	atDur := defaultAccessTokenDuration
	if accessTokenDur != nil {
		atDur = *accessTokenDur
	}
	rtDur := defaultRefreshTokenDuration
	if refreshTokenDur != nil {
		rtDur = *refreshTokenDur
	}
	return &userUseCaseImpl{
		BaseUseCaseImpl:      baseUseCase,
		userRepo:             userRepo,
		logger:               logger,
		tokenGen:             tokenGen,
		accessTokenDuration:  atDur,
		refreshTokenDuration: rtDur,
	}
}

// --- Implement Specific UserUsecase Methods --- //

// Login implements UserUsecase.
// It now accepts and returns types from the schema package.
func (uc *userUseCaseImpl) Login(ctx context.Context, creds schema.LoginCredentials) (*schema.LoginResult, error) {
	uc.logger.Info("Attempting login", "email", creds.Email)

	// 1. Find user by email, check active, check password
	user, err := uc.userRepo.FindByEmail(ctx, creds.Email)
	if err != nil {
		if err.Error() == errUserNotFoundMsg {
			uc.logger.Warn("Login failed: user not found", "email", creds.Email)
			return nil, core_usecase.NewUseCaseError(core_usecase.ErrNotFound, "user not found")
		}
		uc.logger.Error("Failed to find user by email during login", "email", creds.Email, "error", err)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrInternal, "failed to retrieve user data")
	}
	if !user.IsActive {
		uc.logger.Warn("Login failed: user is inactive", "email", creds.Email, "user_id", user.ID)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "user account is inactive")
	}
	if !user.CheckPassword(creds.Password) {
		uc.logger.Warn("Login failed: invalid password", "email", creds.Email, "user_id", user.ID)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "invalid credentials")
	}

	// 4. Prepare custom claims map including the standard "sub" claim
	customClaims := map[string]interface{}{
		"sub":   user.ID.String(),
		"email": user.Email,
		"role":  string(user.Role),
	}

	// 5. Generate JWT token pair using the TokenGenerator interface
	accessToken, refreshToken, expiresAt, err := uc.tokenGen.GenerateTokenPair(
		customClaims,
		uc.accessTokenDuration,
		uc.refreshTokenDuration,
	)
	if err != nil {
		uc.logger.Error("Failed to generate token pair", "user_id", user.ID, "error", err)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrInternal, "failed to generate authentication tokens")
	}

	uc.logger.Info("Login successful", "email", creds.Email, "user_id", user.ID)

	userDTO := schema.UserResponseDTO{
		BaseEntityDTO: core_entity.BaseEntityDTO{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			DeletedAt: user.DeletedAt,
		},
		Username:    user.Username,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Role:        user.Role,
		IsActive:    user.IsActive,
		LastLoginAt: user.LastLoginAt,
		Phone:       user.Phone,
		Address:     user.Address,
		Age:         int(user.Age),
		ProfilePic:  user.ProfilePic,
	}

	// 6. Return LoginResult (using schema type)
	return &schema.LoginResult{
		User:         userDTO,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// Refresh implements UserUsecase.
// It now returns the result type from the schema package.
func (uc *userUseCaseImpl) Refresh(ctx context.Context, refreshToken string) (*schema.RefreshResult, error) {
	uc.logger.Info("Attempting token refresh")

	// 1. Validate refresh token & extract claims
	// TODO: Implement proper refresh token validation
	validatedClaims := map[string]interface{}{"sub": uuid.Nil.String()} // Replace with actual extracted claims
	userIDStr, okSub := validatedClaims["sub"].(string)
	if !okSub {
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "missing or invalid sub claim in refresh token")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "invalid user id format in refresh token sub claim")
	}
	if userID == uuid.Nil {
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "refresh token validation yielded invalid user ID")
	}

	// 2. Load user from DB
	user, err := uc.BaseUseCaseImpl.GetByID(ctx, userID)
	if err != nil || !user.IsActive {
		uc.logger.Warn("User for refresh token not found or inactive", "user_id", userID)
		if err != nil {
			return nil, err
		}
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "invalid user session")
	}

	// 3. Prepare claims for the *new* access token
	newAccessTokenClaims := map[string]interface{}{
		"sub":   user.ID.String(),
		"email": user.Email,
		"role":  string(user.Role),
	}

	// 4. Generate *only* a new access token
	newAccessToken, _, newExpiresAt, err := uc.tokenGen.GenerateTokenPair(
		newAccessTokenClaims,
		uc.accessTokenDuration,
		0, // No new refresh token needed
	)
	if err != nil {
		uc.logger.Error("Failed to generate new access token during refresh", "user_id", user.ID, "error", err)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrInternal, "failed to refresh access token")
	}

	uc.logger.Info("Token refresh successful", "user_id", user.ID)

	// 5. Return RefreshResult (using schema type)
	return &schema.RefreshResult{
		AccessToken:  newAccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    newExpiresAt,
	}, nil
}

/*
// Example implementation for a custom method PromoteUser
func (uc *userUseCaseImpl) PromoteUser(ctx context.Context, userID uuid.UUID, newRole entity.Role) error {
	uc.logger.Info("Promoting user", "user_id", userID, "new_role", newRole)
	if !newRole.IsValid() {
		return core_usecase.NewUseCaseError(core_usecase.ErrInvalidInput, "invalid role specified")
	}

	user, err := uc.GetByID(ctx, userID) // Use embedded GetByID
	if err != nil {
		return err // GetByID already returns UseCaseError
	}

	user.Role = newRole
	// Use the embedded Update method, which expects the DTO from the schema package
	updateDTO := schema.UserUpdateDTO{
		Role: &newRole,
	}
	_, err = uc.Update(ctx, userID, updateDTO) // Use embedded Update
	if err != nil {
		// uc.Update already logs and returns UseCaseError
		return err
	}

	uc.logger.Info("User promotion successful", "user_id", userID, "new_role", newRole)
	return nil
}
*/
