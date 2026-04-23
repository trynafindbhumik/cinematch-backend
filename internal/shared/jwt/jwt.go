package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var jwtSigningKey = []byte("dev-secret-key-change-in-production")

// SetJWTSigningKey sets the signing key for JWT
func SetJWTSigningKey(key string) {
	jwtSigningKey = []byte(key)
}

// Claims represents JWT claims
type Claims struct {
	UserID      int64  `json:"user_id"`
	Email       string `json:"email"`
	IsVerified  bool   `json:"is_verified"`
	IsFirstLogin bool   `json:"is_first_login"`
	jti         string `json:"jti"` // JWT ID for tracking and revocation
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a new JWT access token with jti
func GenerateAccessToken(userID int64, email string, isVerified, isFirstLogin bool) (string, string, error) {
	jti := uuid.New().String() // Generate JWT ID

	claims := Claims{
		UserID:       userID,
		Email:        email,
		IsVerified:   isVerified,
		IsFirstLogin: isFirstLogin,
		jti:          jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "cinematch",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSigningKey)
	return signedToken, jti, err
}

// GenerateRandomRefreshToken generates a random 32-byte refresh token (opaque, not JWT)
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

// GenerateAccessTokenFromJTI creates a new access token with a specific jti (for rotation)
func GenerateAccessTokenFromJTI(userID int64, email string, isVerified, isFirstLogin bool, jti string) (string, error) {
	claims := Claims{
		UserID:       userID,
		Email:        email,
		IsVerified:   isVerified,
		IsFirstLogin: isFirstLogin,
		jti:          jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
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