package internal

// CreateUserRequest represents the request body for creating a new user
type CreateUserRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Role      string `json:"role" validate:"required,oneof=admin user"`

	Phone      string `json:"phone" validate:"omitempty,e164"`
	Address    string `json:"address" validate:"omitempty"`
	Age        int    `json:"age" validate:"omitempty,min=0,max=120"`
	ProfilePic string `json:"profile_pic" validate:"omitempty,url"`
}

// UpdateUserRequest represents the request body for updating an existing user
type UpdateUserRequest struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Password  *string `json:"password,omitempty" validate:"omitempty,min=8"`
}

// UserResponse represents the response body for user data
type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// UserListResponse represents a paginated list of users
type UserListResponse struct {
	Users      []UserResponse `json:"users"`
	TotalCount int64          `json:"total_count"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the response after successful login
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt int64        `json:"expires_at"`
	User      UserResponse `json:"user"`
}
