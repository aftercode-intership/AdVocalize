// backend/internal/config/jwt.go
package config

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	Secret string        `json:"secret"`
	Expiry time.Duration `json:"expiry"`
}

type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Tier   string `json:"tier"`
	jwt.RegisteredClaims
}

func LoadJWTConfig() *JWTConfig {
	secret := os.Getenv("JWT_SECRET")

	// Check if we're in production mode
	isProduction := os.Getenv("ENV") == "production" || os.Getenv("GO_ENV") == "production"

	if secret == "" {
		if isProduction {
			// In production, JWT_SECRET must be explicitly configured
			log.Fatal("FATAL: JWT_SECRET environment variable is required in production. Set JWT_SECRET to a secure random string.")
		}
		// In development, generate a random fallback secret
		log.Println("WARNING: JWT_SECRET not set. Using random development fallback. DO NOT use in production!")
		// Generate a random 32-character secret for development
		secret = generateRandomSecret(32)
	}

	// Validate secret strength
	if isProduction && len(secret) < 32 {
		log.Fatalf("FATAL: JWT_SECRET must be at least 32 characters in production. Current length: %d", len(secret))
	}

	expiry := 24 * time.Hour // 24 hours

	return &JWTConfig{
		Secret: secret,
		Expiry: expiry,
	}
}

func (jc *JWTConfig) GenerateToken(userID, email, tier string) (string, error) {
	claims := &JWTClaims{
		UserID: userID,
		Email:  email,
		Tier:   tier,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jc.Expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "vocalize",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jc.Secret))
}

func (jc *JWTConfig) ValidateToken(tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jc.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// VerifyToken verifies a JWT token using the provided secret string
func VerifyToken(tokenString, secret string) (*JWTClaims, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// GenerateToken generates a JWT token using the provided secret string and expiry hours
func GenerateToken(userID, email, secret string, expiryHours int) (string, error) {
	claims := &JWTClaims{
		UserID: userID,
		Email:  email,
		Tier:   "",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "vocalize",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// generateRandomSecret generates a cryptographically secure random secret
func generateRandomSecret(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based if crypto/rand fails
		for i := range b {
			b[i] = charset[(int(time.Now().UnixNano())+i)%len(charset)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
