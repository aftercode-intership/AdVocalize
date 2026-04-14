package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DatabaseURL string

	// Server
	Port        string
	Environment string
	FrontendURL string

	// JWT
	JWTSecret               string
	JWTExpiryHours          int
	RefreshTokenExpiryHours int

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Email
	SendGridAPIKey string
	SendEmails     bool
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	config := &Config{
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://vocalize:localpassword@localhost:5432/vocalize_db?sslmode=disable"),
		Port:                    getEnv("PORT", "8081"),
		Environment:             getEnv("ENVIRONMENT", "development"),
		FrontendURL:             getEnv("FRONTEND_URL", "http://localhost:3000"),
		JWTSecret:               getEnv("JWT_SECRET", "change_me_in_production"),
		JWTExpiryHours:          getEnvInt("JWT_EXPIRY_HOURS", 24),
		RefreshTokenExpiryHours: getEnvInt("REFRESH_TOKEN_EXPIRY_HOURS", 168),
		GoogleClientID:          getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:      getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:       getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8081/api/auth/google/callback"),
		SendGridAPIKey:          getEnv("SENDGRID_API_KEY", ""),
		SendEmails:              getEnvBool("SEND_EMAILS", false),
	}

	// Validate required fields
	if config.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if config.GoogleClientID == "" || config.GoogleClientSecret == "" {
		fmt.Println("Warning: Google OAuth credentials not set")
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
