package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/trynafindbhumik/cinematch-backend/internal/shared/logger"
)

type Config struct {
	Port           string
	AllowedOrigins []string
	Environment    string
}

var App Config

func Load() {
	if err := godotenv.Load(); err != nil {
		logger.Debug("No .env file found — using environment variables")
	}

	// Environment
	App.Environment = getEnv("ENVIRONMENT", "development")
	App.Port = getEnv("PORT", "8080")

	// CORS allowed origins - comma-separated in env
	originsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:3000")
	App.AllowedOrigins = parseOrigins(originsStr)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseOrigins(origins string) []string {
	var result []string
	for _, o := range strings.Split(origins, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
