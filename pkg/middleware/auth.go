package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// UserClaims represents the custom claims in the JWT
type UserClaims struct {
	ID    string `json:"user_id"`
	Role  string `json:"role"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// JWTConfig holds the configuration for JWT validation
type JWTConfig struct {
	Secret         string
	TokenLookup    string
	TokenHeadName  string
	ContextKey     string
	ExpirationTime time.Duration
	ErrorHandler   fiber.ErrorHandler
}

// DefaultJWTConfig is the default JWT auth configuration
var DefaultJWTConfig = JWTConfig{
	Secret:         "your-secret-key", // Change this in production!
	TokenLookup:    "header:Authorization",
	TokenHeadName:  "Bearer",
	ContextKey:     "user",
	ExpirationTime: 24 * time.Hour,
	ErrorHandler:   defaultErrorHandler,
}

func GenerateToken(userID, role, email string, expirationTime time.Duration) (string, error) {
	claims := UserClaims{
		ID:    userID,
		Role:  role,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "KMT-wqim",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expirationTime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(DefaultJWTConfig.Secret))
}

// defaultErrorHandler is the default error handler
func defaultErrorHandler(c *fiber.Ctx, err error) error {
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
		"error": err.Error(),
	})
}

// AuthMiddleware is a middleware function that validates JWT tokens
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

		// Parse the token
		claims := &UserClaims{}
		parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			// Validate the algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(cfg.Secret), nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
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

// OptionalAuth middleware doesn't require authentication but will load claims if token is present
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
func RequireRole(role string, contextKey ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetClaims(c, contextKey...)
		if claims == nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "authentication required",
			})
		}

		if claims.Role != role {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient permissions",
			})
		}

		return c.Next()
	}
}
