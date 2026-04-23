package models

import "time"

// EmailVerification represents an email verification record in the database
type EmailVerification struct {
	ID        int64      // Primary key
	Email     string     // Email address
	UserID    int64      // Foreign key to users
	Type      string     // "signup" or "password_reset"
	OTPHash   string     // SHA-256 hash of OTP
	TokenHash string     // SHA-256 hash of token
	ExpiresAt time.Time  // Expiry time
	UsedAt    *time.Time // When verification was used
	Attempts  int        // Failed attempt counter
	CreatedAt time.Time  // Record creation time
}

// VerificationType constants
const (
	VerificationTypeSignup        = "signup"
	VerificationTypePasswordReset = "password_reset"
)