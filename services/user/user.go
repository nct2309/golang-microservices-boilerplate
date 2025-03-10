package user

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nct2309/golang-microservices-boilerplate/internal/core"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Role represents user permission levels
type Role string

const (
	// RoleAdmin has full system access
	RoleAdmin Role = "admin"
	// RoleManager has elevated access to manage resources
	RoleManager Role = "manager"
	// RoleUser has standard user access
	RoleUser Role = "user"
	// RoleGuest has limited access
	RoleGuest Role = "guest"
)

// User represents a system user with authentication and authorization data
type User struct {
	core.BaseEntity
	Username    string     `json:"username" gorm:"uniqueIndex;not null"`
	Email       string     `json:"email" gorm:"uniqueIndex;not null"`
	Password    string     `json:"-" gorm:"not null"` // Password is never exposed in JSON
	FirstName   string     `json:"first_name" gorm:"size:50"`
	LastName    string     `json:"last_name" gorm:"size:50"`
	Role        Role       `json:"role" gorm:"type:varchar(20);default:'user';not null"`
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	LastLoginAt *time.Time `json:"last_login_at" gorm:"default:null"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook to validate and prepare data before saving to database
func (u *User) BeforeCreate(tx *gorm.DB) error {
	// Set default ID if not provided
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}

	// Default role if not set
	if u.Role == "" {
		u.Role = RoleUser
	}

	// Extract username from email if not provided
	if u.Username == "" && u.Email != "" {
		parts := strings.Split(u.Email, "@")
		if len(parts) > 0 {
			u.Username = parts[0]
		}
	}

	// Hash password if provided and not already hashed
	if len(u.Password) > 0 && !isHashedPassword(u.Password) {
		hashedPassword, err := HashPassword(u.Password)
		if err != nil {
			return err
		}
		u.Password = hashedPassword
	}

	return nil
}

// Validate performs validation on the user data
func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}

	if !strings.Contains(u.Email, "@") {
		return errors.New("email format is invalid")
	}

	if len(u.Password) < 8 && !isHashedPassword(u.Password) {
		return errors.New("password must be at least 8 characters")
	}

	return nil
}

// SetPassword hashes and sets the user password
func (u *User) SetPassword(plainPassword string) error {
	if len(plainPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	hashedPassword, err := HashPassword(plainPassword)
	if err != nil {
		return err
	}

	u.Password = hashedPassword
	return nil
}

// CheckPassword verifies if the provided password matches the stored hash
func (u *User) CheckPassword(plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainPassword))
	return err == nil
}

// HashPassword generates a bcrypt hash from plain text password
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// isHashedPassword checks if the password is already hashed with bcrypt
func isHashedPassword(password string) bool {
	// Bcrypt hashes have a specific format with cost parameter
	return len(password) == 60 && strings.HasPrefix(password, "$2a$") ||
		strings.HasPrefix(password, "$2b$") || strings.HasPrefix(password, "$2y$")
}

// FullName returns the user's full name
func (u *User) FullName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

// DisplayName returns the most appropriate name to display
func (u *User) DisplayName() string {
	if fullName := u.FullName(); fullName != "" {
		return fullName
	}
	return u.Username
}

// IsAdmin checks if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// HasRole checks if the user has at least the specified role
func (u *User) HasRole(requiredRole Role) bool {
	// Define role hierarchy
	roleHierarchy := map[Role]int{
		RoleAdmin:   100,
		RoleManager: 75,
		RoleUser:    50,
		RoleGuest:   25,
	}

	return roleHierarchy[u.Role] >= roleHierarchy[requiredRole]
}

// UpdateLoginTime updates the user's last login timestamp
func (u *User) UpdateLoginTime() {
	now := time.Now()
	u.LastLoginAt = &now
}
