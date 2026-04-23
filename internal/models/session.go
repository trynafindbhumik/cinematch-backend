package models

import "time"

// Session represents a user session in the database
type Session struct {
	ID               string    // UUID session identifier
	UserID           int64     // Foreign key to users
	RefreshTokenHash string    // SHA-256 hash of refresh token
	DeviceName       string    // Device/client identifier
	UserAgent        string    // Browser/client user agent
	IPAddress        string    // Client IP address
	ExpiresAt        time.Time // Session expiry
	CreatedAt        time.Time // Record creation time
}