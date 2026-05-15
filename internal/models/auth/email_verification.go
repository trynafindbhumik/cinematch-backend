package auth

import "time"

// Email verification and session management models.

type EmailVerification struct {
	ID        int64      `json:"id"`         // Primary key
	Email     string     `json:"email"`      // Email address
	UserID    int64      `json:"user_id"`    // Foreign key to users
	Type      string     `json:"type"`       // "signup" or "password_reset"
	OTPHash   string     `json:"otp_hash"`   // SHA-256 hash of OTP
	TokenHash string     `json:"token_hash"` // SHA-256 hash of token
	ExpiresAt time.Time  `json:"expires_at"` // Expiry time
	UsedAt    *time.Time `json:"used_at"`    // When verification was used
	Attempts  int        `json:"attempts"`   // Failed attempt counter
	CreatedAt time.Time  `json:"created_at"` // Record creation time
}

// Verification type constants
const (
	VerificationTypeSignup         = "signup"           // New user signup verification
	VerificationTypePasswordReset  = "password_reset"   // Password reset verification
	VerificationTypeEmailChangeOld = "email_change_old" // OTP sent to old email for verification
	VerificationTypeEmailChangeNew = "email_change_new" // OTP sent to new email for verification
)
