package config

import (
	"log"
	"os"
)

type AuthConfig struct {
	JWTSigningKey       string
	AccessTokenExpiry  int // minutes
	RefreshTokenExpiry int // days
	OTPExpiry          int // minutes
	OTPMaxAttempts     int
	VerificationExpiry int // minutes
	LockoutDuration    int // minutes
	MaxLoginAttempts   int
	ResendCooldown     int // seconds
}

var Auth AuthConfig

func LoadAuthConfig() {
	Auth.JWTSigningKey = os.Getenv("JWT_SIGNING_KEY")
	if Auth.JWTSigningKey == "" {
		if os.Getenv("ENVIRONMENT") == "production" {
			log.Fatal("JWT_SIGNING_KEY is required in production")
		}
		Auth.JWTSigningKey = "dev-secret-key-change-in-production"
		log.Println("WARNING: Using default JWT_SIGNING_KEY")
	}

	Auth.AccessTokenExpiry = getEnvInt("ACCESS_TOKEN_EXPIRY_MINUTES", 15)
	Auth.RefreshTokenExpiry = getEnvInt("REFRESH_TOKEN_EXPIRY_DAYS", 7)
	Auth.OTPExpiry = getEnvInt("OTP_EXPIRY_MINUTES", 10)
	Auth.OTPMaxAttempts = getEnvInt("OTP_MAX_ATTEMPTS", 5)
	Auth.VerificationExpiry = getEnvInt("VERIFICATION_TOKEN_EXPIRY_MINUTES", 60)
	Auth.LockoutDuration = getEnvInt("LOCKOUT_DURATION_MINUTES", 15)
	Auth.MaxLoginAttempts = getEnvInt("MAX_LOGIN_ATTEMPTS", 5)
	Auth.ResendCooldown = getEnvInt("RESEND_COOLDOWN_SECONDS", 60)
}