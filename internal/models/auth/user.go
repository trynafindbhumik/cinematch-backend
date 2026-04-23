package models

import "time"

// User represents a user in the database
type User struct {
	ID             int64      // Primary key
	PublicID       string     // UUID for external exposure
	Email          string     // Unique email
	PasswordHash   string     // Bcrypt hashed password
	Name           string     // Display name
	Role           string     // "user" or "admin"
	IsVerified     bool       // Email verified flag
	IsFirstLogin   bool       // First login flag
	FailedAttempts int        // Failed login counter
	LockoutUntil   *time.Time // Account lockout expiry
	CreatedAt      time.Time  // Record creation time
	UpdatedAt      time.Time  // Last update time
	IsDeleted      bool       // Soft delete flag
}

// UserRole constants
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)