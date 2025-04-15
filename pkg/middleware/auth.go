package middleware

import (
	"errors"
	"fmt"
	"golang-microservices-boilerplate/pkg/utils"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// UserClaims represents the custom claims in the JWT
type UserClaims struct {
	Data map[string]interface{} `json:"data,omitempty"` // Holds custom claims
	jwt.RegisteredClaims
}

// JWTConfig holds the configuration for JWT validation
type JWTConfig struct {
	AccessTokenSecret  string // Secret key for Access Tokens
	RefreshTokenSecret string // Separate secret key for Refresh Tokens
	TokenLookup        string
	TokenHeadName      string
	ContextKey         string
	ExpirationTime     time.Duration // Default duration for Access Tokens
	ErrorHandler       fiber.ErrorHandler
}

// DefaultJWTConfig is the default JWT auth configuration
var DefaultJWTConfig = JWTConfig{
	AccessTokenSecret:  utils.GetEnv("ACCESS_TOKEN_SECRET", "access_token_secret_wqim"),
	RefreshTokenSecret: utils.GetEnv("REFRESH_TOKEN_SECRET", "refresh_token_secret_KMT"), // CHANGE THIS!
	TokenLookup:        "header:Authorization",
	TokenHeadName:      "Bearer",
	ContextKey:         "user",
	ExpirationTime:     24 * time.Hour, // Default access token expiry (e.g., 1 hour)
	ErrorHandler:       defaultErrorHandler,
}

// GenerateToken creates a JWT token with the given details and secret.
// It expects the User ID to be within customClaims under the key "sub".
func GenerateToken(customClaims map[string]interface{}, expirationTime time.Duration, secret string) (string, error) {
	// Extract Subject (User ID) from claims map
	subject := "" // Default to empty string
	if sub, ok := customClaims["sub"].(string); ok {
		subject = sub
	}
	// Optionally, remove "sub" from customClaims if you don't want it duplicated in Data
	// delete(customClaims, "sub")

	claims := UserClaims{
		Data: customClaims,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,                             // Use 'sub' claim for User ID extracted from map
			Issuer:    utils.GetEnv("APP_NAME", "wqimKMT"), // Consider making this configurable
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expirationTime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateTokenPair creates both an access token and a refresh token for a user.
// It expects the User ID to be within customClaims under the key "sub".
func GenerateTokenPair(customClaims map[string]interface{}, accessDuration, refreshDuration time.Duration, accessSecret, refreshSecret string) (accessToken, refreshToken string, expiresAt int64, err error) {
	// Generate Access Token
	accessTokenExp := time.Now().Add(accessDuration)
	accessToken, err = GenerateToken(customClaims, accessDuration, accessSecret)
	if err != nil {
		err = errors.New("failed to generate access token: " + err.Error())
		return
	}

	// Generate Refresh Token
	// Consider if refresh token needs different/simpler claims
	refreshToken, err = GenerateToken(customClaims, refreshDuration, refreshSecret)
	if err != nil {
		err = errors.New("failed to generate refresh token: " + err.Error())
		return
	}

	expiresAt = accessTokenExp.Unix()
	return
}

// defaultErrorHandler is the default error handler
func defaultErrorHandler(c *fiber.Ctx, err error) error {
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
		"error": err.Error(),
	})
}

// AuthMiddleware is a middleware function that validates ACCESS tokens using the primary Secret
func AuthMiddleware(config ...JWTConfig) fiber.Handler {
	// Set default config
	cfg := DefaultJWTConfig

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]
	}

	// Return middleware handler
	return func(c *fiber.Ctx) error {
		// Get token from the request
		token, err := extractToken(c, cfg)
		if err != nil {
			return cfg.ErrorHandler(c, err)
		}

		// Parse the token using the primary Secret (for access tokens)
		claims := &UserClaims{}
		parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			// Validate the algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			// Use the primary secret for access token validation
			return []byte(cfg.AccessTokenSecret), nil
		})

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return cfg.ErrorHandler(c, errors.New("token expired"))
			} else if errors.Is(err, jwt.ErrSignatureInvalid) {
				return cfg.ErrorHandler(c, errors.New("invalid token signature"))
			}
			return cfg.ErrorHandler(c, errors.New("invalid token"))
		}

		if !parsedToken.Valid {
			return cfg.ErrorHandler(c, errors.New("invalid token"))
		}

		// Check if token is expired
		if claims.ExpiresAt != nil {
			if claims.ExpiresAt.Time.Before(time.Now()) {
				return cfg.ErrorHandler(c, errors.New("token expired"))
			}
		}

		// Store user information in context
		c.Locals(cfg.ContextKey, claims)

		// Proceed to the next middleware or handler
		return c.Next()
	}
}

// extractToken extracts the token from the request based on the lookup configuration
func extractToken(c *fiber.Ctx, config JWTConfig) (string, error) {
	parts := strings.Split(config.TokenLookup, ":")
	if len(parts) != 2 {
		return "", errors.New("invalid token lookup config")
	}

	lookupPart := strings.TrimSpace(parts[0])
	lookupKey := strings.TrimSpace(parts[1])

	switch lookupPart {
	case "header":
		// Get token from header
		authHeader := c.Get(lookupKey)
		if authHeader == "" {
			return "", errors.New("missing auth header")
		}

		// Check for token head (e.g., "Bearer ")
		if config.TokenHeadName != "" {
			token := strings.TrimPrefix(authHeader, config.TokenHeadName+" ")
			if token == authHeader {
				return "", errors.New("invalid token format")
			}
			return token, nil
		}
		return authHeader, nil

	case "query":
		// Get token from query parameter
		token := c.Query(lookupKey)
		if token == "" {
			return "", errors.New("missing auth query parameter")
		}
		return token, nil

	case "cookie":
		// Get token from cookie
		token := c.Cookies(lookupKey)
		if token == "" {
			return "", errors.New("missing auth cookie")
		}
		return token, nil

	default:
		return "", errors.New("unsupported token lookup method")
	}
}

// OptionalAuth middleware doesn't require authentication but will load claims if a valid ACCESS token is present
func OptionalAuth(config ...JWTConfig) fiber.Handler {
	// Set default config
	cfg := DefaultJWTConfig

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]
	}

	// Custom error handler that just continues the chain
	optionalErrorHandler := func(c *fiber.Ctx, err error) error {
		return c.Next()
	}

	// Use the original config but with our optional error handler
	cfg.ErrorHandler = optionalErrorHandler

	// Use the regular auth middleware with our modified config
	return AuthMiddleware(cfg)
}

// GetClaims is a helper function to get user claims from context
func GetClaims(c *fiber.Ctx, contextKey ...string) *UserClaims {
	key := DefaultJWTConfig.ContextKey
	if len(contextKey) > 0 {
		key = contextKey[0]
	}

	claims, ok := c.Locals(key).(*UserClaims)
	if !ok {
		return nil
	}
	return claims
}

// RequireRole middleware ensures the authenticated user has the required role
func RequireRole(roles []string, contextKey ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetClaims(c, contextKey...)
		if claims == nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "authentication required",
			})
		}

		// Look for role in the Data map
		roleClaim, ok := claims.Data["role"].(string) // Assuming role is stored as a string with key "role"
		if !ok {
			// Role claim missing or not a string
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintln("role claim missing or invalid format in token"),
			})
		}

		// lowercase the roleClaim
		roleClaim = strings.ToLower(roleClaim)
		// lowercase the roles
		for i, role := range roles {
			roles[i] = strings.ToLower(role)
		}

		if !slices.Contains(roles, roleClaim) {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient permissions",
			})
		}

		return c.Next()
	}
}

// --- Refresh Token Specific Logic (Example Placeholder) ---

// ValidateRefreshToken specifically validates a refresh token using the refresh secret.
// Note: This should likely live in the usecase or a dedicated auth service, not middleware.
func ValidateRefreshToken(tokenString string, refreshSecret string) (*UserClaims, error) {
	claims := &UserClaims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		// Use the REFRESH secret for validation
		return []byte(refreshSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("refresh token expired")
		} else if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, errors.New("invalid refresh token signature")
		}
		return nil, errors.New("invalid refresh token")
	}

	if !parsedToken.Valid {
		return nil, errors.New("invalid refresh token")
	}

	// Optional: Check against a revocation list here if implementing

	return claims, nil
}
