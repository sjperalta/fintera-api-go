package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port        string
	Environment string

	// Database
	DatabaseURL string

	// JWT
	JWTSecret          string
	JWTExpirationHours int

	// Storage
	StoragePath string

	// Background Workers
	WorkerCount int

	// CORS
	AllowedOrigins []string

	// Email (Resend)
	ResendAPIKey string
	FromEmail    string

	// Sentry
	SentryDSN string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		Environment:        getEnv("ENVIRONMENT", "development"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		StoragePath:        getEnv("STORAGE_PATH", "./storage"),
		WorkerCount:        getEnvAsInt("WORKER_COUNT", 5),
		AllowedOrigins:     getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),
		ResendAPIKey:       getEnv("RESEND_API_KEY", ""),
		FromEmail:          getEnv("FROM_EMAIL", "noreply@fintera.app"),
		SentryDSN:          getEnv("SENTRY_DSN", ""),
	}

	// Validate required configuration
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" && cfg.Environment == "production" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}

	// Set default JWT secret for development
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-secret-change-in-production"
	}

	return cfg, nil
}

// getEnv reads an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt reads an environment variable as integer
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsSlice reads an environment variable as comma-separated slice
func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return strings.Split(valueStr, ",")
}
