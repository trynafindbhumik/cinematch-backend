package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mssola/useragent"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
	authmodels "github.com/trynafindbhumik/cinematch-backend/internal/models/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/hash"
)

var ErrUserNotFound = errors.New("user not found")
var ErrVerificationNotFound = errors.New("verification not found")

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// CreateUser creates a new unverified user
func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, name string) (*authmodels.User, error) {
	user := &authmodels.User{}

	err := db.Pool().QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, role, is_verified)
		VALUES ($1, $2, $3, 'user', false)
		RETURNING id, public_id, email, password_hash, name, role, is_verified
	`, email, passwordHash, name).Scan(
		&user.ID, &user.PublicID, &user.Email, &user.PasswordHash,
		&user.Name, &user.Role, &user.IsVerified,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves user by email
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*authmodels.User, error) {
	user := &authmodels.User{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, public_id, email, password_hash, name, role, is_verified, is_first_login, failed_attempts, lockout_until
		FROM users 
		WHERE email = $1 AND is_deleted = false
	`, email).Scan(
		&user.ID, &user.PublicID, &user.Email, &user.PasswordHash,
		&user.Name, &user.Role, &user.IsVerified, &user.IsFirstLogin, &user.FailedAttempts, &user.LockoutUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves user by ID
func (r *Repository) GetUserByID(ctx context.Context, userID int64) (*authmodels.User, error) {
	user := &authmodels.User{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, public_id, email, password_hash, name, role, is_verified, is_first_login, failed_attempts, lockout_until
		FROM users 
		WHERE id = $1 AND is_deleted = false
	`, userID).Scan(
		&user.ID, &user.PublicID, &user.Email, &user.PasswordHash,
		&user.Name, &user.Role, &user.IsVerified, &user.IsFirstLogin, &user.FailedAttempts, &user.LockoutUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// SetUserVerified marks a user as verified
func (r *Repository) SetUserVerified(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET is_verified = true WHERE id = $1
	`, userID)
	return err
}

// CreateEmailVerification creates a new email verification record
func (r *Repository) CreateEmailVerification(ctx context.Context, ev *authmodels.EmailVerification) (int64, error) {
	var id int64

	err := db.Pool().QueryRow(ctx, `
		INSERT INTO email_verifications (email, user_id, type, otp_hash, token_hash, expires_at, attempts)
		VALUES ($1, $2, $3, $4, $5, $6, 0)
		RETURNING id
	`, ev.Email, ev.UserID, ev.Type, ev.OTPHash, ev.TokenHash, ev.ExpiresAt).Scan(&id)

	return id, err
}

// GetEmailVerification retrieves email verification by ID
func (r *Repository) GetEmailVerification(ctx context.Context, id int64) (*authmodels.EmailVerification, error) {
	ev := &authmodels.EmailVerification{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, email, user_id, type, otp_hash, token_hash, expires_at, used_at, attempts
		FROM email_verifications
		WHERE id = $1
	`, id).Scan(
		&ev.ID, &ev.Email, &ev.UserID, &ev.Type,
		&ev.OTPHash, &ev.TokenHash, &ev.ExpiresAt, &ev.UsedAt, &ev.Attempts,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}
		return nil, err
	}

	return ev, nil
}

// GetEmailVerificationByToken retrieves email verification by token hash
func (r *Repository) GetEmailVerificationByToken(ctx context.Context, tokenHash string) (*authmodels.EmailVerification, error) {
	ev := &authmodels.EmailVerification{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, email, user_id, type, otp_hash, token_hash, expires_at, used_at, attempts
		FROM email_verifications
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&ev.ID, &ev.Email, &ev.UserID, &ev.Type,
		&ev.OTPHash, &ev.TokenHash, &ev.ExpiresAt, &ev.UsedAt, &ev.Attempts,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}
		return nil, err
	}

	return ev, nil
}

// GetLatestEmailVerificationByUserID retrieves the most recent verification for a user
func (r *Repository) GetLatestEmailVerificationByUserID(ctx context.Context, userID int64) (*authmodels.EmailVerification, error) {
	ev := &authmodels.EmailVerification{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, email, user_id, type, otp_hash, token_hash, expires_at, used_at, attempts, created_at
		FROM email_verifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&ev.ID, &ev.Email, &ev.UserID, &ev.Type,
		&ev.OTPHash, &ev.TokenHash, &ev.ExpiresAt, &ev.UsedAt, &ev.Attempts, &ev.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}
		return nil, err
	}

	return ev, nil
}

// UpdateEmailVerificationAttempts updates the attempts counter
func (r *Repository) UpdateEmailVerificationAttempts(ctx context.Context, id int64, attempts int) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE email_verifications SET attempts = $1 WHERE id = $2
	`, attempts, id)
	return err
}

// MarkEmailVerificationUsed marks verification as used
func (r *Repository) MarkEmailVerificationUsed(ctx context.Context, id int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE email_verifications SET used_at = $1 WHERE id = $2
	`, time.Now(), id)
	return err
}

// DeleteEmailVerification deletes a verification record
func (r *Repository) DeleteEmailVerification(ctx context.Context, id int64) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM email_verifications WHERE id = $1
	`, id)
	return err
}

// DeleteEmailVerificationByUserID deletes all verification records for a user
func (r *Repository) DeleteEmailVerificationByUserID(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM email_verifications WHERE user_id = $1
	`, userID)
	return err
}

// CreateSession creates a new session for user with refresh token and device info
func (r *Repository) CreateSession(ctx context.Context, userID int64, refreshToken, deviceName, userAgent, ipAddress string) (string, error) {
	var sessionID string
	refreshTokenHash := hash.HashSHA256(refreshToken)

	// Parse user agent to extract device info
	parsedDeviceName := ParseDevice(userAgent)
	if deviceName != "" {
		parsedDeviceName = deviceName + " - " + parsedDeviceName
	}

	err := db.Pool().QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, device_name, user_agent, ip_address, expires_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, '')::inet, now() + interval '7 days')
		RETURNING id::text
	`, userID, refreshTokenHash, parsedDeviceName, userAgent, ipAddress).Scan(&sessionID)

	return sessionID, err
}

// ParseDevice parses User-Agent string and returns formatted device info
func ParseDevice(userAgent string) string {
	if userAgent == "" {
		return "Unknown"
	}

	ua := useragent.New(userAgent)

	browserName, _ := ua.Browser()

	deviceType := "Desktop"
	if ua.Mobile() {
		deviceType = "Mobile"
	}

	if browserName != "" {
		return fmt.Sprintf("%s - %s", deviceType, browserName)
	}

	return deviceType
}

// IncrementFailedAttempts increments the failed login attempts for a user
func (r *Repository) IncrementFailedAttempts(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET failed_attempts = failed_attempts + 1 WHERE id = $1
	`, userID)
	return err
}

// LockoutUser sets a lockout time for a user
func (r *Repository) LockoutUser(ctx context.Context, userID int64, lockoutUntil time.Time) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET lockout_until = $1, failed_attempts = 0 WHERE id = $2
	`, lockoutUntil, userID)
	return err
}

// ResetFailedAttempts resets failed login attempts for a user
func (r *Repository) ResetFailedAttempts(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET failed_attempts = 0, lockout_until = NULL WHERE id = $1
	`, userID)
	return err
}

// DeleteSessionByRefreshToken deletes a session by refresh token hash
func (r *Repository) DeleteSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	refreshTokenHash := hash.HashSHA256(refreshToken)
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM sessions WHERE refresh_token_hash = $1
	`, refreshTokenHash)
	return err
}

// UpdateUserPassword updates user's password hash
func (r *Repository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET password_hash = $1 WHERE id = $2
	`, passwordHash, userID)
	return err
}

// DeleteAllUserSessions deletes all sessions for a user
func (r *Repository) DeleteAllUserSessions(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM sessions WHERE user_id = $1
	`, userID)
	return err
}