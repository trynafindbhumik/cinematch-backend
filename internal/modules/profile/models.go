package profile

import "time"

// EmailChangeRequest stores pending email change with verification state
type EmailChangeRequest struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	OldEmail  string    `json:"oldEmail"`
	NewEmail  string    `json:"newEmail"`
	Step      string    `json:"step"` // "pending_old_verification", "pending_new_verification", "completed"
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// AccountStatus represents user account state
type AccountStatus struct {
	IsDisabled    bool       `json:"isDisabled"`
	DisabledUntil *time.Time `json:"disabledUntil,omitempty"`
	IsDeleted     bool       `json:"isDeleted"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}
