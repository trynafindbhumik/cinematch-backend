package jwt

// JWT token generation and validation.
// Handles access tokens with claims and refresh tokens.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trynafindbhumik/cinematch-backend/internal/config"
)

var jwtSigningKey = []byte("dev-secret-key-change-in-production")

// SetJWTSigningKey sets the signing key for JWT
func SetJWTSigningKey(key string) {
	jwtSigningKey = []byte(key)
}

// Claims represents JWT claims with user data
type Claims struct {
	UserID       int64  `json:"user_id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	IsVerified   bool   `json:"is_verified"`
	IsFirstLogin bool   `json:"is_first_login"`
	JTI          string `json:"jti"` // JWT ID for tracking and revocation
	jti          string
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a new JWT access token with unique JTI
func GenerateAccessToken(userID int64, email, role string, isVerified, isFirstLogin bool) (string, string, error) {
	jti := uuid.New().String()

	claims := Claims{
		UserID:       userID,
		Email:        email,
		Role:         role,
		IsVerified:   isVerified,
		IsFirstLogin: isFirstLogin,
		JTI:          jti,
		jti:          jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.Auth.AccessTokenExpiry) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "cinematch",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSigningKey)
	return signedToken, jti, err
}

// GenerateRandomRefreshToken generates a random 32-byte refresh token
func GenerateRandomRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateVerificationToken generates a random token for email verification links
func GenerateVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateAccessTokenFromJTI creates a new access token with a specific JTI
func GenerateAccessTokenFromJTI(userID int64, email, role string, isVerified, isFirstLogin bool, jti string) (string, error) {
	claims := Claims{
		UserID:       userID,
		Email:        email,
		Role:         role,
		IsVerified:   isVerified,
		IsFirstLogin: isFirstLogin,
		JTI:          jti,
		jti:          jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.Auth.AccessTokenExpiry) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "cinematch",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSigningKey)
}

// ValidateAccessToken validates and parses a JWT access token
func ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSigningKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ExtractJTIFromToken extracts the JTI from a token without full validation
func ExtractJTIFromToken(tokenString string) (string, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return "", fmt.Errorf("invalid claims type")
	}

	if claims.JTI != "" {
		return claims.JTI, nil
	}

	if claims.ID != "" {
		return claims.ID, nil
	}

	return "", fmt.Errorf("no JTI found in token")
}

// GetTokenIssuedAt extracts the issued-at time from a token
// Used for user revocation check after password change
func GetTokenIssuedAt(tokenString string) (time.Time, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid claims type")
	}

	if claims.IssuedAt == nil {
		return time.Time{}, fmt.Errorf("no issued-at time in token")
	}

	return claims.IssuedAt.Time, nil
}
