package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

var client *redis.Client

const (
	// Key prefixes for Redis
	VerificationPrefix         = "verification:"
	VerificationByUserPrefix   = "verification:user:"
	VerificationByTokenPrefix  = "verification:token:"
	VerificationByEmailPrefix  = "verification:email:"
	MagicLinkPrefix           = "magic_link:"
	MagicLinkByTokenPrefix    = "magic_link:token:"
)

func Load() error {
	godotenv.Load()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return fmt.Errorf("REDIS_URL environment variable not set")
	}

	// Parse the Redis URL properly
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	// Override timeouts for fast fail
	opt.DialTimeout = 2 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second
	opt.PoolSize = 10
	opt.MinIdleConns = 2

	client = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		client = nil
		return fmt.Errorf("redis connection failed: %w", err)
	}

	logger.Info("Redis connected successfully")
	return nil
}

func Client() *redis.Client {
	return client
}

func Close() {
	if client != nil {
		client.Close()
	}
}

// VerificationData stores OTP/verification data in Redis
type VerificationData struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`
	OTPHash   string    `json:"otp_hash"`
	TokenHash string    `json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}

// SetVerification stores verification data with TTL (for OTP expiry)
func SetVerification(ctx context.Context, verificationID string, data *VerificationData, ttl time.Duration) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	key := VerificationPrefix + verificationID

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal verification data: %w", err)
	}

	if err := client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store verification: %w", err)
	}

	// Also index by user_id for resend cooldown check
	userKey := VerificationByUserPrefix + fmt.Sprintf("%d", data.UserID)
	// Store mapping: user -> verificationID (overwrite old one)
	if err := client.Set(ctx, userKey, verificationID, ttl).Err(); err != nil {
		logger.Warn("Failed to index verification by user", logger.Err(err))
	}

	// Index by token hash for lookup during verify
	if data.TokenHash != "" {
		tokenKey := VerificationByTokenPrefix + data.TokenHash
		if err := client.Set(ctx, tokenKey, verificationID, ttl).Err(); err != nil {
			logger.Warn("Failed to index verification by token", logger.Err(err))
		}
	}

	// Index by email for lookup during forgot password
	if data.Email != "" && data.Type == "password_reset" {
		emailKey := VerificationByEmailPrefix + data.Email
		if err := client.Set(ctx, emailKey, verificationID, ttl).Err(); err != nil {
			logger.Warn("Failed to index verification by email", logger.Err(err))
		}
	}

	return nil
}

// GetVerification retrieves verification data by ID
func GetVerification(ctx context.Context, verificationID string) (*VerificationData, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	key := VerificationPrefix + verificationID

	result, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get verification: %w", err)
	}

	var data VerificationData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal verification data: %w", err)
	}

	return &data, nil
}

// GetVerificationByToken retrieves verification data by token hash
func GetVerificationByToken(ctx context.Context, tokenHash string) (*VerificationData, string, error) {
	if client == nil {
		return nil, "", fmt.Errorf("redis client not initialized")
	}
	tokenKey := VerificationByTokenPrefix + tokenHash

	verificationID, err := client.Get(ctx, tokenKey).Result()
	if err == redis.Nil {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to get verification ID by token: %w", err)
	}

	data, err := GetVerification(ctx, verificationID)
	if err != nil {
		return nil, "", err
	}

	return data, verificationID, nil
}

// GetVerificationByEmail retrieves verification data by email (for password reset)
func GetVerificationByEmail(ctx context.Context, email string) (*VerificationData, string, error) {
	if client == nil {
		return nil, "", fmt.Errorf("redis client not initialized")
	}
	emailKey := VerificationByEmailPrefix + email

	verificationID, err := client.Get(ctx, emailKey).Result()
	if err == redis.Nil {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to get verification ID by email: %w", err)
	}

	data, err := GetVerification(ctx, verificationID)
	if err != nil {
		return nil, "", err
	}

	return data, verificationID, nil
}

// GetLatestVerificationByUserID retrieves the latest verification ID for a user
func GetLatestVerificationByUserID(ctx context.Context, userID int64) (string, error) {
	if client == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	userKey := VerificationByUserPrefix + fmt.Sprintf("%d", userID)

	verificationID, err := client.Get(ctx, userKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get latest verification ID: %w", err)
	}

	return verificationID, nil
}

// UpdateVerificationAttempts updates the attempts counter
func UpdateVerificationAttempts(ctx context.Context, verificationID string, attempts int) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	data, err := GetVerification(ctx, verificationID)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("verification not found")
	}

	data.Attempts = attempts

	// Get remaining TTL
	key := VerificationPrefix + verificationID
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}

	if ttl > 0 {
		return SetVerification(ctx, verificationID, data, ttl)
	}

	return fmt.Errorf("verification expired")
}

// DeleteVerification deletes verification data and all indices
func DeleteVerification(ctx context.Context, verificationID string) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	// Get data first to clean up indices
	data, err := GetVerification(ctx, verificationID)
	if err != nil {
		return err
	}

	// Delete main key
	key := VerificationPrefix + verificationID
	if err := client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete verification: %w", err)
	}

	// Clean up user index
	if data != nil && data.UserID > 0 {
		userKey := VerificationByUserPrefix + fmt.Sprintf("%d", data.UserID)
		client.Del(ctx, userKey)
	}

	// Clean up token index
	if data != nil && data.TokenHash != "" {
		tokenKey := VerificationByTokenPrefix + data.TokenHash
		client.Del(ctx, tokenKey)
	}

	// Clean up email index
	if data != nil && data.Email != "" && data.Type == "password_reset" {
		emailKey := VerificationByEmailPrefix + data.Email
		client.Del(ctx, emailKey)
	}

	return nil
}

// DeleteVerificationByUserID deletes all verification for a user
func DeleteVerificationByUserID(ctx context.Context, userID int64) error {
	verificationID, err := GetLatestVerificationByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if verificationID != "" {
		return DeleteVerification(ctx, verificationID)
	}
	return nil
}

// MagicLinkData stores magic link data in Redis
type MagicLinkData struct {
	ID        string `json:"id"`
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
}

// SetMagicLink stores magic link data with TTL (5 minutes)
func SetMagicLink(ctx context.Context, token string, data *MagicLinkData, ttl time.Duration) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	key := MagicLinkPrefix + token

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal magic link data: %w", err)
	}

	if err := client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store magic link: %w", err)
	}

	// Also index by user_id for lookup
	userKey := MagicLinkByTokenPrefix + fmt.Sprintf("%d", data.UserID)
	if err := client.Set(ctx, userKey, token, ttl).Err(); err != nil {
		logger.Warn("Failed to index magic link by user", logger.Err(err))
	}

	return nil
}

// GetMagicLink retrieves magic link data by token
func GetMagicLink(ctx context.Context, token string) (*MagicLinkData, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	key := MagicLinkPrefix + token

	result, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get magic link: %w", err)
	}

	var data MagicLinkData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal magic link data: %w", err)
	}

	return &data, nil
}

// GetMagicLinkByUserID retrieves the latest magic link token for a user
func GetMagicLinkByUserID(ctx context.Context, userID int64) (string, error) {
	if client == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	userKey := MagicLinkByTokenPrefix + fmt.Sprintf("%d", userID)

	token, err := client.Get(ctx, userKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get magic link token by user: %w", err)
	}

	return token, nil
}

// DeleteMagicLink deletes magic link data
func DeleteMagicLink(ctx context.Context, token string) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	// Get data first to clean up user index
	data, err := GetMagicLink(ctx, token)
	if err != nil {
		return err
	}

	key := MagicLinkPrefix + token
	if err := client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete magic link: %w", err)
	}

	// Clean up user index
	if data != nil && data.UserID > 0 {
		userKey := MagicLinkByTokenPrefix + fmt.Sprintf("%d", data.UserID)
		client.Del(ctx, userKey)
	}

	return nil
}

// DeleteMagicLinkByUserID deletes magic link for a user
func DeleteMagicLinkByUserID(ctx context.Context, userID int64) error {
	token, err := GetMagicLinkByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if token != "" {
		return DeleteMagicLink(ctx, token)
	}
	return nil
}
