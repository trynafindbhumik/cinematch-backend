package profile

// Profile repository for database operations.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrNoChangesDetected  = errors.New("no changes detected")
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// GetUserByID retrieves user by ID
func (r *Repository) GetUserByID(ctx context.Context, userID int64) (*User, error) {
	user := &User{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, public_id, email, name, tag, profile_url, is_verified, is_first_login, smart_suggest, role, is_disabled, disabled_until, previously_disabled_at
		FROM users 
		WHERE id = $1 AND is_deleted = false
	`, userID).Scan(
		&user.ID, &user.PublicID, &user.Email, &user.Name,
		&user.Tag, &user.ProfileURL, &user.IsVerified, &user.IsFirstLogin, &user.SmartSuggest, &user.Role,
		&user.IsDisabled, &user.DisabledUntil, &user.PreviouslyDisabledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// User represents user data for profile operations
type User struct {
	ID                   int64
	PublicID             string
	Email                string
	Name                 string
	Tag                  string
	ProfileURL           *string
	IsVerified           bool
	IsFirstLogin         bool
	SmartSuggest         bool
	PasswordHash         string
	Role                 string
	IsDisabled           bool
	DisabledUntil        *time.Time
	PreviouslyDisabledAt *time.Time
}

// GetUserPasswordHash retrieves user's password hash for verification
func (r *Repository) GetUserPasswordHash(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := db.Pool().QueryRow(ctx, `
		SELECT password_hash FROM users WHERE id = $1 AND is_deleted = false
	`, userID).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	return hash, nil
}

// UpdateProfile updates user's name, profile URL, and smart suggest settings
func (r *Repository) UpdateProfile(ctx context.Context, userID int64, name string, profileURL *string, smartSuggest *bool) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users 
		SET name = COALESCE(NULLIF($2, ''), name),
		    profile_url = COALESCE($3, profile_url),
		    smart_suggest = COALESCE($4, smart_suggest),
		    updated_at = NOW()
		WHERE id = $1 AND is_deleted = false
	`, userID, name, profileURL, smartSuggest)
	return err
}

// VerifyPassword verifies the user's current password
func (r *Repository) VerifyPassword(ctx context.Context, userID int64, passwordHash string) (bool, error) {
	var storedHash string
	err := db.Pool().QueryRow(ctx, `
		SELECT password_hash FROM users WHERE id = $1 AND is_deleted = false
	`, userID).Scan(&storedHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrUserNotFound
		}
		return false, err
	}
	return storedHash == passwordHash, nil
}

// UpdatePassword updates user's password hash
func (r *Repository) UpdatePassword(ctx context.Context, userID int64, newPasswordHash string) error {
	result, err := db.Pool().Exec(ctx, `
		UPDATE users 
		SET password_hash = $2, updated_at = NOW()
		WHERE id = $1 AND is_deleted = false
	`, userID, newPasswordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// CheckEmailExists checks if email is already registered (excluding current user)
func (r *Repository) CheckEmailExists(ctx context.Context, email string, excludeUserID int64) (bool, error) {
	var exists bool
	err := db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users 
			WHERE LOWER(email) = LOWER($1) AND is_deleted = false AND id != $2
		)
	`, email, excludeUserID).Scan(&exists)
	return exists, err
}

// DisableAccount temporarily disables user account
func (r *Repository) DisableAccount(ctx context.Context, userID int64, disabledUntil time.Time) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users 
		SET is_disabled = true, disabled_until = $2, previously_disabled_at = CASE WHEN is_disabled = false THEN NOW() ELSE previously_disabled_at END, updated_at = NOW()
		WHERE id = $1 AND is_deleted = false
	`, userID, disabledUntil)
	return err
}

// EnableAccount re-enables a disabled user account
func (r *Repository) EnableAccount(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users 
		SET is_disabled = false, disabled_until = NULL, updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

// SoftDeleteAccount marks account as deleted but retains data
func (r *Repository) SoftDeleteAccount(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users 
		SET is_deleted = true, deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

// CreateEmailChangeRequest creates a pending email change request record
func (r *Repository) CreateEmailChangeRequest(ctx context.Context, userID int64, oldEmail, newEmail string) (*EmailChange, error) {
	var id int64
	err := db.Pool().QueryRow(ctx, `
		INSERT INTO email_change_requests (user_id, old_email, new_email, step, expires_at)
		VALUES ($1, $2, $3, 'pending_old_verification', NOW() + INTERVAL '15 minutes')
		RETURNING id
	`, userID, oldEmail, newEmail).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &EmailChange{ID: id, UserID: userID, OldEmail: oldEmail, NewEmail: newEmail, Step: "pending_old_verification"}, nil
}

// GetEmailChangeRequest retrieves an email change request by ID
func (r *Repository) GetEmailChangeRequest(ctx context.Context, id int64) (*EmailChange, error) {
	ecr := &EmailChange{}
	err := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, old_email, new_email, step, expires_at, created_at
		FROM email_change_requests
		WHERE id = $1
	`, id).Scan(&ecr.ID, &ecr.UserID, &ecr.OldEmail, &ecr.NewEmail, &ecr.Step, &ecr.ExpiresAt, &ecr.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("email change request not found")
		}
		return nil, err
	}
	return ecr, nil
}

// GetEmailChangeRequestByUserID retrieves the latest email change request for a user
func (r *Repository) GetEmailChangeRequestByUserID(ctx context.Context, userID int64) (*EmailChange, error) {
	ecr := &EmailChange{}
	err := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, old_email, new_email, step, otp_hash, expires_at, created_at
		FROM email_change_requests
		WHERE user_id = $1 AND step != 'completed'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&ecr.ID, &ecr.UserID, &ecr.OldEmail, &ecr.NewEmail, &ecr.Step, &ecr.OTPHash, &ecr.ExpiresAt, &ecr.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("no pending email change request")
		}
		return nil, err
	}
	return ecr, nil
}

// UpdateEmailChangeStep updates the step of an email change request
func (r *Repository) UpdateEmailChangeStep(ctx context.Context, id int64, step string) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE email_change_requests SET step = $2 WHERE id = $1
	`, id, step)
	return err
}

// UpdateEmailChangeWithOTP stores the OTP hash and advances to next step
func (r *Repository) UpdateEmailChangeWithOTP(ctx context.Context, id int64, step string, otpHash string) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE email_change_requests SET step = $2, otp_hash = $3 WHERE id = $1
	`, id, step, otpHash)
	return err
}

// UpdateUserEmail updates the user's email address
func (r *Repository) UpdateUserEmail(ctx context.Context, userID int64, newEmail string) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET email = $2, updated_at = NOW() WHERE id = $1
	`, userID, newEmail)
	return err
}

// DeleteEmailChangeRequest removes an email change request
func (r *Repository) DeleteEmailChangeRequest(ctx context.Context, id int64) error {
	_, err := db.Pool().Exec(ctx, `DELETE FROM email_change_requests WHERE id = $1`, id)
	return err
}

// DeleteAllUserSessions removes all sessions for a user
func (r *Repository) DeleteAllUserSessions(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// EmailChange represents a pending email change request
type EmailChange struct {
	ID        int64
	UserID    int64
	OldEmail  string
	NewEmail  string
	Step      string
	OTPHash   string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CleanupDeletedAccounts permanently removes accounts that have been soft-deleted for more than X days
func (r *Repository) CleanupDeletedAccounts(ctx context.Context, retentionDays int) (int64, error) {
	result, err := db.Pool().Exec(ctx, `
		DELETE FROM users 
		WHERE is_deleted = true 
		AND deleted_at < NOW() - INTERVAL '1 day' * $1
	`, retentionDays)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ValidateProfileUpdate checks if there are any actual changes to make
func (r *Repository) ValidateProfileUpdate(name string, profileURL *string) bool {
	return (name != "" || profileURL != nil)
}

// ClearProfilePicture sets the user's profile_url to NULL
func (r *Repository) ClearProfilePicture(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users
		SET profile_url = NULL, updated_at = NOW()
		WHERE id = $1 AND is_deleted = false
	`, userID)
	return err
}
