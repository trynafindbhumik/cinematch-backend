package auth

// User model representing a registered user in the system.

import "time"

// User represents a registered user in the database
type User struct {
	ID                   int64      `json:"id"`                     // Primary key
	PublicID             string     `json:"public_id"`              // UUID for external exposure
	Email                string     `json:"email"`                  // Unique email
	PasswordHash         string     `json:"-"`                      // Bcrypt hashed password
	Name                 string     `json:"name"`                   // Display name
	Role                 string     `json:"role"`                   // "user" or "admin"
	Tag                  string     `json:"tag"`                    // User tag (screen_enthusiast, cinema_lover, etc.)
	ProfileURL           *string    `json:"profile_url"`            // Profile photo URL
	IsVerified           bool       `json:"is_verified"`            // Email verified flag
	IsFirstLogin         bool       `json:"is_first_login"`         // First login flag
	FailedAttempts       int        `json:"failed_attempts"`        // Failed login counter
	LockoutUntil         *time.Time `json:"lockout_until"`          // Account lockout expiry
	CreatedAt            time.Time  `json:"created_at"`             // Record creation time
	UpdatedAt            time.Time  `json:"updated_at"`             // Last update time
	IsDeleted            bool       `json:"is_deleted"`             // Soft delete flag
	DeletedAt            *time.Time `json:"deleted_at"`             // Soft delete timestamp
	DeletionScheduledAt  *time.Time `json:"deletion_scheduled_at"`  // Scheduled deletion time
	SmartSuggest         bool       `json:"smart_suggest"`          // Smart suggestion flag
	IsDisabled           bool       `json:"is_disabled"`            // Account disabled flag
	DisabledUntil        *time.Time `json:"disabled_until"`         // Temporary disable until
	PreviouslyDisabledAt *time.Time `json:"previously_disabled_at"` // Previously disabled timestamp
}

// User role constants
const (
	RoleUser  = "user"  // Regular user
	RoleAdmin = "admin" // Administrator
)

// User tag constants (achievement levels)
const (
	TagScreenEnthusiast = "screen_enthusiast" // New user
	TagCinemaLover      = "cinema_lover"      // Active user
	TagCinephile        = "cinephile"         // Regular user
	TagCinephilePro     = "cinephile_pro"     // Power user
	TagCinephileElite   = "cinephile_elite"   // Elite user
)
