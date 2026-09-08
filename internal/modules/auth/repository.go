package auth

// Authentication repository for database and cache operations.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mssola/useragent"
	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/db"
	"github.com/trynafindbhumik/cinematch-backend/internal/models"
	authmodels "github.com/trynafindbhumik/cinematch-backend/internal/models/auth"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/hash"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/redis"
)

var ErrUserNotFound = errors.New("user not found")
var ErrVerificationNotFound = errors.New("verification not found")
var ErrMaxSessionsReached = errors.New("maximum sessions reached")

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// CreateUser inserts new user into database
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

// GetUserByEmail finds user by email address
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*authmodels.User, error) {
	user := &authmodels.User{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, public_id, email, password_hash, name, role, is_verified, is_first_login, failed_attempts, lockout_until, is_disabled, disabled_until
		FROM users
		WHERE email = $1 AND is_deleted = false
	`, email).Scan(
		&user.ID, &user.PublicID, &user.Email, &user.PasswordHash,
		&user.Name, &user.Role, &user.IsVerified, &user.IsFirstLogin, &user.FailedAttempts, &user.LockoutUntil,
		&user.IsDisabled, &user.DisabledUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// GetUserByID finds user by ID
func (r *Repository) GetUserByID(ctx context.Context, userID int64) (*authmodels.User, error) {
	user := &authmodels.User{}

	err := db.Pool().QueryRow(ctx, `
		SELECT id, public_id, email, password_hash, name, role, is_verified, is_first_login, failed_attempts, lockout_until, is_disabled, disabled_until
		FROM users
		WHERE id = $1 AND is_deleted = false
	`, userID).Scan(
		&user.ID, &user.PublicID, &user.Email, &user.PasswordHash,
		&user.Name, &user.Role, &user.IsVerified, &user.IsFirstLogin, &user.FailedAttempts, &user.LockoutUntil,
		&user.IsDisabled, &user.DisabledUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// SetUserVerified marks user as email verified
func (r *Repository) SetUserVerified(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET is_verified = true WHERE id = $1
	`, userID)
	return err
}

// SetUserFirstLoginFalse marks user first login as completed (false)
func (r *Repository) SetUserFirstLoginFalse(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET is_first_login = false WHERE id = $1
	`, userID)
	return err
}

// SetUserUnverified marks user as email unverified
func (r *Repository) SetUserUnverified(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET is_verified = false WHERE id = $1
	`, userID)
	return err
}

// CreateEmailVerification stores verification data in Redis with TTL
func (r *Repository) CreateEmailVerification(ctx context.Context, ev *authmodels.EmailVerification) (string, error) {
	verificationID := uuid.New().String()

	ttl := time.Duration(config.Auth.OTPExpiry) * time.Minute

	redisData := &redis.VerificationData{
		Email:     ev.Email,
		UserID:    ev.UserID,
		Type:      ev.Type,
		OTPHash:   ev.OTPHash,
		TokenHash: ev.TokenHash,
		ExpiresAt: ev.ExpiresAt,
		Attempts:  0,
		CreatedAt: time.Now(),
	}

	err := redis.SetVerification(ctx, verificationID, redisData, ttl)
	if err != nil {
		return "", fmt.Errorf("failed to create verification in redis: %w", err)
	}

	return verificationID, nil
}

// GetEmailVerification retrieves verification data from Redis
func (r *Repository) GetEmailVerification(ctx context.Context, id string) (*authmodels.EmailVerification, error) {
	data, err := redis.GetVerification(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get verification from redis: %w", err)
	}
	if data == nil {
		return nil, ErrVerificationNotFound
	}

	return &authmodels.EmailVerification{
		ID:        data.ID,
		Email:     data.Email,
		UserID:    data.UserID,
		Type:      data.Type,
		OTPHash:   data.OTPHash,
		TokenHash: data.TokenHash,
		ExpiresAt: data.ExpiresAt,
		Attempts:  data.Attempts,
		CreatedAt: data.CreatedAt,
	}, nil
}

// GetEmailVerificationByToken finds verification by token hash
func (r *Repository) GetEmailVerificationByToken(ctx context.Context, tokenHash string) (*authmodels.EmailVerification, error) {
	data, _, err := redis.GetVerificationByToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get verification by token from redis: %w", err)
	}
	if data == nil {
		return nil, ErrVerificationNotFound
	}

	ev := &authmodels.EmailVerification{
		Email:     data.Email,
		UserID:    data.UserID,
		Type:      data.Type,
		OTPHash:   data.OTPHash,
		TokenHash: data.TokenHash,
		ExpiresAt: data.ExpiresAt,
		Attempts:  data.Attempts,
		CreatedAt: data.CreatedAt,
	}

	return ev, nil
}

// GetEmailVerificationByTokenWithID finds verification by token hash and returns ID
func (r *Repository) GetEmailVerificationByTokenWithID(ctx context.Context, tokenHash string) (*authmodels.EmailVerification, string, error) {
	data, verificationID, err := redis.GetVerificationByToken(ctx, tokenHash)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get verification by token from redis: %w", err)
	}
	if data == nil {
		return nil, "", ErrVerificationNotFound
	}

	ev := &authmodels.EmailVerification{
		Email:     data.Email,
		UserID:    data.UserID,
		Type:      data.Type,
		OTPHash:   data.OTPHash,
		TokenHash: data.TokenHash,
		ExpiresAt: data.ExpiresAt,
		Attempts:  data.Attempts,
		CreatedAt: data.CreatedAt,
	}

	return ev, verificationID, nil
}

// GetLatestEmailVerificationByUserID finds most recent verification for user
func (r *Repository) GetLatestEmailVerificationByUserID(ctx context.Context, userID int64) (*authmodels.EmailVerification, string, error) {
	verificationID, err := redis.GetLatestVerificationByUserID(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get latest verification id from redis: %w", err)
	}
	if verificationID == "" {
		return nil, "", ErrVerificationNotFound
	}

	data, err := redis.GetVerification(ctx, verificationID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get verification from redis: %w", err)
	}
	if data == nil {
		return nil, "", ErrVerificationNotFound
	}

	ev := &authmodels.EmailVerification{
		Email:     data.Email,
		UserID:    data.UserID,
		Type:      data.Type,
		OTPHash:   data.OTPHash,
		TokenHash: data.TokenHash,
		ExpiresAt: data.ExpiresAt,
		Attempts:  data.Attempts,
		CreatedAt: data.CreatedAt,
	}

	return ev, verificationID, nil
}

// UpdateEmailVerificationAttempts increments failed attempt counter
func (r *Repository) UpdateEmailVerificationAttempts(ctx context.Context, id string, attempts int) error {
	return redis.UpdateVerificationAttempts(ctx, id, attempts)
}

// MarkEmailVerificationUsed marks verification as used (no-op for Redis)
func (r *Repository) MarkEmailVerificationUsed(ctx context.Context, id string) error {
	return nil
}

// DeleteEmailVerification removes verification from Redis
func (r *Repository) DeleteEmailVerification(ctx context.Context, id string) error {
	return redis.DeleteVerification(ctx, id)
}

// DeleteEmailVerificationByUserID removes all user verifications from Redis
func (r *Repository) DeleteEmailVerificationByUserID(ctx context.Context, userID int64) error {
	return redis.DeleteVerificationByUserID(ctx, userID)
}

// CreateSession creates new session with refresh token
// Enforces max 5 sessions per user by rejecting new sessions if limit is reached
func (r *Repository) CreateSession(ctx context.Context, userID int64, refreshToken, jti, deviceName, userAgent, ipAddress string) (string, error) {
	var sessionID string
	refreshTokenHash := hash.HashSHA256(refreshToken)
	var jtiHash *string
	if jti != "" {
		h := hash.HashSHA256(jti)
		jtiHash = &h
	}

	// Parse user agent to identify device type
	parsedDeviceName := ParseDevice(userAgent)
	if deviceName != "" {
		parsedDeviceName = deviceName + " - " + parsedDeviceName
	}

	// Check current session count
	var sessionCount int
	err := db.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE user_id = $1 AND (expires_at IS NULL OR expires_at > now())
	`, userID).Scan(&sessionCount)
	if err != nil {
		return "", fmt.Errorf("failed to count sessions: %w", err)
	}

	if sessionCount >= 5 {
		return "", ErrMaxSessionsReached
	}

	err = db.Pool().QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, jti_hash, device_name, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, '')::inet, now() + interval '7 days')
		RETURNING id::text
	`, userID, refreshTokenHash, jtiHash, parsedDeviceName, userAgent, ipAddress).Scan(&sessionID)

	return sessionID, err
}

// ParseDevice extracts device info from User-Agent string
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

// IncrementFailedAttempts adds 1 to failed login counter
func (r *Repository) IncrementFailedAttempts(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET failed_attempts = failed_attempts + 1 WHERE id = $1
	`, userID)
	return err
}

// LockoutUser temporarily disables account for specified time
func (r *Repository) LockoutUser(ctx context.Context, userID int64, lockoutUntil time.Time) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET lockout_until = $1, failed_attempts = 0 WHERE id = $2
	`, lockoutUntil, userID)
	return err
}

// ResetFailedAttempts clears failed attempt counter and lockout
func (r *Repository) ResetFailedAttempts(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET failed_attempts = 0, lockout_until = NULL WHERE id = $1
	`, userID)
	return err
}

// DeleteSessionByRefreshToken removes session using refresh token hash
func (r *Repository) DeleteSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	refreshTokenHash := hash.HashSHA256(refreshToken)
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM sessions WHERE refresh_token_hash = $1
	`, refreshTokenHash)
	return err
}

// GetSessionByRefreshToken finds session by refresh token hash
func (r *Repository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	refreshTokenHash := hash.HashSHA256(refreshToken)
	sess := &models.Session{}
	err := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, refresh_token_hash, jti_hash, device_name, user_agent, ip_address::text, expires_at, created_at, last_used_at
		FROM sessions
		WHERE refresh_token_hash = $1 AND expires_at > now()
	`, refreshTokenHash).Scan(
		&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.JTIHash,
		&sess.DeviceName, &sess.UserAgent, &sess.IPAddress,
		&sess.ExpiresAt, &sess.CreatedAt, &sess.LastUsedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return sess, nil
}

// GetSessionByJTIHash finds session by JWT ID hash
func (r *Repository) GetSessionByJTIHash(ctx context.Context, jtiHash string) (*models.Session, error) {
	sess := &models.Session{}
	err := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, refresh_token_hash, jti_hash, device_name, user_agent, ip_address::text, expires_at, created_at, last_used_at
		FROM sessions
		WHERE jti_hash = $1 AND (expires_at IS NULL OR expires_at > now())
	`, jtiHash).Scan(
		&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.JTIHash,
		&sess.DeviceName, &sess.UserAgent, &sess.IPAddress,
		&sess.ExpiresAt, &sess.CreatedAt, &sess.LastUsedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by jti: %w", err)
	}
	return sess, nil
}

// UpdateSessionJTIHash updates JWT ID hash for session
func (r *Repository) UpdateSessionJTIHash(ctx context.Context, sessionID string, jti string) error {
	jtiHash := hash.HashSHA256(jti)
	_, err := db.Pool().Exec(ctx, `
		UPDATE sessions SET jti_hash = $1, last_used_at = now()
		WHERE id = $2
	`, jtiHash, sessionID)
	return err
}

// RotateRefreshToken updates refresh token hash and extends expiry
func (r *Repository) RotateRefreshToken(ctx context.Context, sessionID string, newRefreshToken string) error {
	refreshTokenHash := hash.HashSHA256(newRefreshToken)
	_, err := db.Pool().Exec(ctx, `
		UPDATE sessions SET refresh_token_hash = $1, expires_at = now() + interval '30 days'
		WHERE id = $2
	`, refreshTokenHash, sessionID)
	return err
}

// RotateRefreshTokenByUserID rotates refresh token for any active session of the user
// Used during email verification when user already has a session
func (r *Repository) RotateRefreshTokenByUserID(ctx context.Context, userID int64, newRefreshToken string) error {
	refreshTokenHash := hash.HashSHA256(newRefreshToken)
	_, err := db.Pool().Exec(ctx, `
		UPDATE sessions SET refresh_token_hash = $1, expires_at = now() + interval '30 days'
		WHERE id = (
			SELECT id FROM sessions
			WHERE user_id = $2 AND (expires_at IS NULL OR expires_at > now())
			ORDER BY created_at DESC
			LIMIT 1
		)
	`, refreshTokenHash, userID)
	return err
}

// EnableAccount re-enables a disabled user account during login
func (r *Repository) EnableAccount(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET is_disabled = false, disabled_until = NULL, updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

// UpdateUserPassword updates user's password hash
func (r *Repository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := db.Pool().Exec(ctx, `
		UPDATE users SET password_hash = $1 WHERE id = $2
	`, passwordHash, userID)
	return err
}

// DeleteAllUserSessions removes all sessions for user
func (r *Repository) DeleteAllUserSessions(ctx context.Context, userID int64) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM sessions WHERE user_id = $1
	`, userID)
	return err
}

// GetAllUserSessions retrieves all active sessions for a user
func (r *Repository) GetAllUserSessions(ctx context.Context, userID int64) ([]*models.Session, error) {
	rows, err := db.Pool().Query(ctx, `
		SELECT id, user_id, refresh_token_hash, jti_hash, device_name, user_agent, ip_address::text, expires_at, created_at, last_used_at
		FROM sessions
		WHERE user_id = $1 AND (expires_at IS NULL OR expires_at > now())
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		sess := &models.Session{}
		err := rows.Scan(
			&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.JTIHash,
			&sess.DeviceName, &sess.UserAgent, &sess.IPAddress,
			&sess.ExpiresAt, &sess.CreatedAt, &sess.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// GetSessionByID retrieves a session by its ID
func (r *Repository) GetSessionByID(ctx context.Context, sessionID string) (*models.Session, error) {
	sess := &models.Session{}
	err := db.Pool().QueryRow(ctx, `
		SELECT id, user_id, refresh_token_hash, jti_hash, device_name, user_agent, ip_address::text, expires_at, created_at, last_used_at
		FROM sessions
		WHERE id = $1 AND (expires_at IS NULL OR expires_at > now())
	`, sessionID).Scan(
		&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.JTIHash,
		&sess.DeviceName, &sess.UserAgent, &sess.IPAddress,
		&sess.ExpiresAt, &sess.CreatedAt, &sess.LastUsedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by ID: %w", err)
	}
	return sess, nil
}

// DeleteSessionByID removes a session by its ID
func (r *Repository) DeleteSessionByID(ctx context.Context, sessionID string) error {
	_, err := db.Pool().Exec(ctx, `
		DELETE FROM sessions WHERE id = $1
	`, sessionID)
	return err
}

// CreateMagicLink stores magic link data in Redis
func (r *Repository) CreateMagicLink(ctx context.Context, token string, data *redis.MagicLinkData, ttl time.Duration) error {
	return redis.SetMagicLink(ctx, token, data, ttl)
}

// GetMagicLink retrieves magic link data from Redis
func (r *Repository) GetMagicLink(ctx context.Context, token string) (*redis.MagicLinkData, error) {
	return redis.GetMagicLink(ctx, token)
}

// DeleteMagicLink removes magic link from Redis
func (r *Repository) DeleteMagicLink(ctx context.Context, token string) error {
	return redis.DeleteMagicLink(ctx, token)
}
