package models

// Session model for user authentication sessions.

import "time"

// Session represents a user session in the database
type Session struct {
	ID               string    `json:"id"`           // UUID session identifier
	UserID           int64     `json:"user_id"`      // Foreign key to users
	RefreshTokenHash string    `json:"-"`            // SHA-256 hash of refresh token
	JTIHash          *string   `json:"-"`            // SHA-256 hash of JWT ID for token revocation
	DeviceName       string    `json:"device_name"`  // Device/client identifier
	UserAgent        string    `json:"user_agent"`   // Browser/client user agent
	IPAddress        string    `json:"ip_address"`   // Client IP address
	ExpiresAt        time.Time `json:"expires_at"`   // Session expiry
	CreatedAt        time.Time `json:"created_at"`   // Record creation time
	LastUsedAt       time.Time `json:"last_used_at"` // Last time session was used
}
