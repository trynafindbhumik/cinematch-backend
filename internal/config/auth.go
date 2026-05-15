package config

import (
	"os"
	"strconv"

	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

type AuthConfig struct {
	JWTSigningKey      string
	AccessTokenExpiry  int // minutes
	RefreshTokenExpiry int // days
	OTPExpiry          int // minutes
	OTPMaxAttempts     int
	VerificationExpiry  int // minutes
	LockoutDuration    int // minutes
	MaxLoginAttempts   int
	ResendCooldown     int // seconds
	MagicLinkExpiry    int // minutes
}

var Auth AuthConfig

func LoadAuthConfig() {
	Auth.JWTSigningKey = os.Getenv("JWT_SIGNING_KEY")
	if Auth.JWTSigningKey == "" {
		if os.Getenv("ENVIRONMENT") == "production" {
			logger.Fatal("JWT_SIGNING_KEY is required in production", logger.String("environment", "production"))
		}
		Auth.JWTSigningKey = "dev-secret-key-change-in-production"
		logger.Warn("Using default JWT_SIGNING_KEY - CHANGE IN PRODUCTION")
	}

	Auth.AccessTokenExpiry = getEnvInt("ACCESS_TOKEN_EXPIRY_MINUTES", 15)
	Auth.RefreshTokenExpiry = getEnvInt("REFRESH_TOKEN_EXPIRY_DAYS", 30)
	Auth.OTPExpiry = getEnvInt("OTP_EXPIRY_MINUTES", 10)
	Auth.OTPMaxAttempts = getEnvInt("OTP_MAX_ATTEMPTS", 5)
	Auth.VerificationExpiry = getEnvInt("VERIFICATION_TOKEN_EXPIRY_MINUTES", 60)
	Auth.LockoutDuration = getEnvInt("LOCKOUT_DURATION_MINUTES", 15)
	Auth.MaxLoginAttempts = getEnvInt("MAX_LOGIN_ATTEMPTS", 5)
	Auth.ResendCooldown = getEnvInt("RESEND_COOLDOWN_SECONDS", 60)
	Auth.MagicLinkExpiry = getEnvInt("MAGIC_LINK_EXPIRY_MINUTES", 5)
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
