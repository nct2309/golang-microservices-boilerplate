package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	core_logger "golang-microservices-boilerplate/pkg/core/logger"
	core_usecase "golang-microservices-boilerplate/pkg/core/usecase"
	"golang-microservices-boilerplate/pkg/middleware"
	"golang-microservices-boilerplate/pkg/utils"
	"golang-microservices-boilerplate/services/user-service/internal/entity"
	user_repository "golang-microservices-boilerplate/services/user-service/internal/repository"
	"golang-microservices-boilerplate/services/user-service/internal/schema"

	// Remove unused schema import
	// schema "golang-microservices-boilerplate/services/user-service/internal/schema"

	"github.com/google/uuid"
)

// Specific error messages to check against repository errors
const errUserNotFoundMsg = "entity not found"

// Define JWT expiration durations (can be configured externally)
const (
	defaultAccessTokenDuration  = 7 * 24 * time.Hour  // 7 days
	defaultRefreshTokenDuration = 30 * 24 * time.Hour // 30 days
)

// LoginCredentials, LoginResult, RefreshResult are now defined in the schema package
// type LoginCredentials struct { ... }
// type LoginResult struct { ... }
// type RefreshResult struct { ... }

// UserUsecase defines the business logic operations for users.
// Login/Refresh results are handled differently now
type UserUsecase interface {
	// Embed the core use case, now without DTO generics
	core_usecase.BaseUseCase[entity.User]
	// Login returns entity and token details directly, uses locally defined LoginCredentials
	Login(ctx context.Context, creds schema.LoginCredentials) (*schema.LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*schema.RefreshResult, error)
	// PromoteUser(ctx context.Context, userID uuid.UUID, newRole entity.Role) error // Example custom method
}

// userUseCaseImpl implements the UserUsecase interface.
type userUseCaseImpl struct {
	// Embed the core use case implementation, now without DTO generics
	*core_usecase.BaseUseCaseImpl[entity.User]
	userRepo             user_repository.UserRepository
	logger               core_logger.Logger
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

// NewUserUseCase creates a new instance of UserUsecase.
func NewUserUseCase(
	userRepo user_repository.UserRepository,
	logger core_logger.Logger,
	accessTokenDur *time.Duration,
	refreshTokenDur *time.Duration,
) UserUsecase { // Return the UserUsecase interface type
	// Remove DTO generics when creating the base use case
	baseUseCase := core_usecase.NewBaseUseCase(userRepo, logger)
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
		accessTokenDuration:  atDur,
		refreshTokenDuration: rtDur,
	}
}

// --- Implement Specific UserUsecase Methods --- //

// Login implements UserUsecase.
// Modified to return *entity.User and token details directly
// Uses locally defined LoginCredentials struct
func (uc *userUseCaseImpl) Login(ctx context.Context, creds schema.LoginCredentials) (*schema.LoginResult, error) {
	uc.logger.Info("Attempting login", "email", creds.Email)

	// 1. Find user by email, check active, check password
	user, err := uc.userRepo.FindByEmail(ctx, creds.Email)
	if err != nil {
		if err.Error() == errUserNotFoundMsg {
			uc.logger.Warn("Login failed: user not found", "email", creds.Email)
			// Return nils and zero values for tokens along with the error
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
	accessToken, refreshToken, expiresAt, err := middleware.GenerateTokenPair(
		customClaims,
		uc.accessTokenDuration,
		uc.refreshTokenDuration,
		utils.GetEnv("ACCESS_TOKEN_SECRET", "access_token_secret_wqim"),
		utils.GetEnv("REFRESH_TOKEN_SECRET", "refresh_token_secret_KMT"),
	)
	if err != nil {
		uc.logger.Error("Failed to generate token pair", "user_id", user.ID, "error", err)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrInternal, "failed to generate authentication tokens")
	}

	uc.logger.Info("Login successful", "email", creds.Email, "user_id", user.ID)

	// 6. Return LoginResult (using schema type)
	// Return the entity and token details directly
	// 	return user, accessToken, refreshToken, expiresAt, nil
	return &schema.LoginResult{
		User:         *user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// Refresh implements UserUsecase.
// It now returns the result type from the schema package.
// TODO: Refactor Refresh to not depend on schema.RefreshResult if schema package is removed
// Uses locally defined RefreshResult struct
func (uc *userUseCaseImpl) Refresh(ctx context.Context, refreshToken string) (*schema.RefreshResult, error) {
	validatedClaims, err := middleware.ValidateRefreshToken(refreshToken, utils.GetEnv("REFRESH_TOKEN_SECRET", "refresh_token_secret_KMT"))
	if err != nil {
		// Wrap the error for consistency
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, fmt.Sprintf("invalid refresh token: %v", err))
	}

	userIDStr, okSub := validatedClaims.Data["sub"].(string)
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

	// 2. Load user from DB using the embedded GetByID
	// The returned 'user' is *entity.User because the BaseUseCaseImpl is specialized
	user, err := uc.BaseUseCaseImpl.GetByID(ctx, userID)
	if err != nil {
		// Handle GetByID errors (which might already be UseCaseError types)
		uc.logger.Warn("User for refresh token not found or GetByID failed", "user_id", userID, "error", err)
		// Check if it was a standard 'not found' or another error
		var ucErr *core_usecase.UseCaseError
		if errors.As(err, &ucErr) && ucErr.Type == core_usecase.ErrNotFound {
			return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "invalid user session")
		}
		// Return the original error if it wasn't ErrNotFound or wrap it
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrInternal, "failed to retrieve user data for refresh")
	}
	// Check if user is active *after* confirming user is not nil
	if !user.IsActive {
		uc.logger.Warn("User for refresh token is inactive", "user_id", userID)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrUnauthorized, "user account is inactive")
	}

	// 3. Prepare claims for the *new* access token (using the fetched user)
	newAccessTokenClaims := map[string]interface{}{
		"sub":   user.ID.String(),
		"email": user.Email,
		"role":  string(user.Role),
	}

	// 4. Generate *only* a new access token
	newAccessToken, _, newExpiresAt, err := middleware.GenerateTokenPair(
		newAccessTokenClaims,
		uc.accessTokenDuration,
		uc.refreshTokenDuration,
		utils.GetEnv("ACCESS_TOKEN_SECRET", "access_token_secret_wqim"),
		utils.GetEnv("REFRESH_TOKEN_SECRET", "refresh_token_secret_KMT"),
	)
	if err != nil {
		uc.logger.Error("Failed to generate new access token during refresh", "user_id", user.ID, "error", err)
		return nil, core_usecase.NewUseCaseError(core_usecase.ErrInternal, "failed to refresh access token")
	}

	uc.logger.Info("Token refresh successful", "user_id", user.ID)

	// 5. Return RefreshResult (using locally defined type)
	return &schema.RefreshResult{
		AccessToken:  newAccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    newExpiresAt,
	}, nil
}
