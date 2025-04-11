package schema

type LoginCredentials struct {
	Email    string
	Password string
}

// LoginResult holds the data returned upon successful login
type LoginResult struct {
	User         UserResponseDTO
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64 // Unix timestamp for access token expiry
}

// RefreshResult holds the data returned upon successful token refresh
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64 // Unix timestamp for new access token expiry
}
