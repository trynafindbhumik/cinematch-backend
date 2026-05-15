package main

import (
	"fmt"
	"os"
	"time"

	"github.com/trynafindbhumik/cinematch-backend/internal/config"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/jwt"
)

func main() {
	// Load config
	config.Load()
	config.LoadAuthConfig()
	jwt.SetJWTSigningKey(config.Auth.JWTSigningKey)

	// Generate token for user 48
	token, _, err := jwt.GenerateAccessToken(48, "bhumikjain925@gmail.com", "user", true, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(token)
	
	// Print expiry time
	expiry := time.Now().Add(time.Duration(config.Auth.AccessTokenExpiry) * time.Minute)
	fmt.Printf("Expires at: %s\n", expiry.Format(time.RFC3339))
}