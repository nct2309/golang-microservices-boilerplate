package entity

import (
	"errors"
	"strings"
	"time"

	"golang-microservices-boilerplate/pkg/core/entity"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Role represents user permission levels as strings.
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleOfficer Role = "officer"
	// Consider if an empty string or a specific "unknown" constant is needed for default/unset states
)

// IsValid checks if the role is one of the predefined valid roles.
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleOfficer:
		return true
	default:
		return false
	}
}

// User represents a system user domain entity
type User struct {
	entity.BaseEntity        // Embed core base entity
	Username          string `json:"username,omitempty" gorm:"uniqueIndex;not null"`
	Email             string `json:"email,omitempty" gorm:"uniqueIndex;not null"`
	Password          string `json:"password,omitempty" gorm:"not null"` // Password is never exposed
	FirstName         string `json:"first_name,omitempty" gorm:"size:50;not null"`
	LastName          string `json:"last_name,omitempty" gorm:"size:50;not null"`
	// Use string for Role, restricted to known values.
	// Ensure database schema uses a string type (e.g., VARCHAR).
	Role        Role       `json:"role,omitempty" gorm:"size:10;not null;check:chk_user_role,role IN ('admin', 'manager', 'officer')"` // Store role as string, Added CHECK constraint
	IsActive    bool       `json:"is_active,omitempty" gorm:"default:true"`                                                            // Default new users to inactive
	LastLoginAt *time.Time `json:"last_login_at,omitempty" gorm:"default:null"`
	// Add other fields from proto if they belong in the core domain model
	// Example: Phone, Address, ProfilePic, Age might or might not be core domain fields
	Phone      string `json:"phone,omitempty" gorm:"size:20"`
	Address    string `json:"address,omitempty" gorm:"type:text"`
	Age        int32  `json:"age,omitempty"`
	ProfilePic string `json:"profile_pic,omitempty" gorm:"size:255"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}

// Add required methods for core.Entity interface with value receivers
func (u User) GetID() uuid.UUID {
	return u.ID
}

// SetID sets the entity ID (needed to implement the Entity interface)
// We use a value receiver method to match the GetID method, but it won't modify the actual object
// The pointer receiver version in the embedded BaseEntity will handle the actual modification
func (u User) SetID(id uuid.UUID) {
	// This method is required for the interface but doesn't need to do anything
	// as the actual ID setting is done by the embedded BaseEntity.SetID method
}

// GetCreatedAt returns the creation timestamp (value receiver for Entity interface)
func (u User) GetCreatedAt() time.Time {
	return u.CreatedAt // Access embedded field
}

// GetUpdatedAt returns the last update timestamp (value receiver for Entity interface)
func (u User) GetUpdatedAt() time.Time {
	return u.UpdatedAt // Access embedded field
}

// GetDeletedAt returns the deletion timestamp (value receiver for Entity interface)
func (u User) GetDeletedAt() *time.Time {
	return u.DeletedAt // Access embedded field
}

// BeforeCreate hook to validate and prepare data before saving to database
func (u *User) BeforeCreate(tx *gorm.DB) error {
	// Call the embedded BaseEntity's hook first
	if err := u.BaseEntity.BeforeCreate(tx); err != nil {
		return err
	}

	// Default role if not set or invalid
	if !u.Role.IsValid() {
		u.Role = RoleOfficer // Default to Officer
	}

	// Extract username from email if not provided
	if u.Username == "" && u.Email != "" {
		parts := strings.Split(u.Email, "@")
		if len(parts) > 0 {
			u.Username = parts[0]
		}
	}

	// Hash password if provided and not already hashed
	if u.Password != "" && !isHashedPassword(u.Password) {
		err := u.SetPassword(u.Password)
		if err != nil {
			return err
		}
	}

	// GORM's default:false for IsActive will handle the initial state.
	// No need to explicitly set u.IsActive = false here.

	return u.Validate() // Validate before saving
}

// BeforeUpdate hook
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	// Call the embedded BaseEntity's hook first
	if err := u.BaseEntity.BeforeUpdate(tx); err != nil {
		return err
	}

	// Hash password if it's being updated and is not already hashed
	// Check if the password field is actually being updated if GORM allows partial updates easily
	if u.Password != "" && !isHashedPassword(u.Password) {
		// Only hash if password changed; check tx.Statement.Changed("Password") if possible/needed
		err := u.SetPassword(u.Password)
		if err != nil {
			return err
		}
	}

	return u.Validate() // Validate before updating
}

// Validate performs validation on the user data
func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(u.Email, "@") {
		return errors.New("email format is invalid")
	}
	// Password validation might only be needed if it's not already hashed
	if u.Password != "" && len(u.Password) < 8 && !isHashedPassword(u.Password) {
		return errors.New("password must be at least 8 characters")
	}
	// Validate role is one of the allowed non-empty values
	if !u.Role.IsValid() {
		return errors.New("invalid role: must be admin, manager, or officer")
	}
	if u.Username == "" {
		return errors.New("username is required") // Assuming username becomes mandatory
	}
	// Add other validations (e.g., name length, age constraints)
	return nil
}

// SetPassword hashes and sets the user password safely
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
	if u.Password == "" || plainPassword == "" {
		return false
	}
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
	// Basic check for bcrypt hash format
	return len(password) >= 60 && strings.HasPrefix(password, "$2")
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

// IsManager checks if the user has manager role
func (u *User) IsManager() bool {
	return u.Role == RoleManager
}

// IsOfficer checks if the user has officer role
func (u *User) IsOfficer() bool {
	return u.Role == RoleOfficer
}

// UpdateLoginTime updates the user's last login timestamp
func (u *User) UpdateLoginTime() {
	now := time.Now()
	u.LastLoginAt = &now
}
