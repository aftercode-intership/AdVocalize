// backend/internal/config/jwt_test.go
package config

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// Test GenerateToken creates a token
func TestGenerateToken_TokenCreation(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, err := jwtConfig.GenerateToken("user-123", "test@example.com", "free")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

// Test GenerateToken includes claims
func TestGenerateToken_ClaimsInclusion(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	userID := "user-456"
	email := "user@test.com"
	tier := "pro"

	token, err := jwtConfig.GenerateToken(userID, email, tier)
	assert.NoError(t, err)

	// Parse and verify claims
	claims := &JWTClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtConfig.Secret), nil
	})

	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, tier, claims.Tier)
}

// Test ValidateToken with valid token
func TestValidateToken_ValidToken(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	claims, err := jwtConfig.ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "free", claims.Tier)
}

// Test ValidateToken with expired token
func TestValidateToken_ExpiredToken(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: -time.Hour, // Already expired
	}

	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	claims, err := jwtConfig.ValidateToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// Test ValidateToken with tampered token
func TestValidateToken_TamperedToken(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	// Tamper with the token
	tamperedToken := token + "extra-data"

	claims, err := jwtConfig.ValidateToken(tamperedToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// Test ValidateToken with invalid signature
func TestValidateToken_InvalidSignature(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	// Create token with different secret
	otherConfig := &JWTConfig{
		Secret: "different-secret-key",
		Expiry: 24 * time.Hour,
	}
	token, _ := otherConfig.GenerateToken("user-123", "test@example.com", "free")

	// Try to validate with our config
	claims, err := jwtConfig.ValidateToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// Test JWTConfig initialization
func TestJWTConfig_Initialization(t *testing.T) {
	config := &JWTConfig{
		Secret: "my-secret-key",
		Expiry: 24 * time.Hour,
	}

	assert.Equal(t, "my-secret-key", config.Secret)
	assert.Equal(t, 24*time.Hour, config.Expiry)
}

// Test JWTClaims structure
func TestJWTClaims_Structure(t *testing.T) {
	claims := &JWTClaims{
		UserID: "user-123",
		Email:  "test@example.com",
		Tier:   "enterprise",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "vocalize",
		},
	}

	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "enterprise", claims.Tier)
	assert.Equal(t, "vocalize", claims.Issuer)
}

// Test token expiration
func TestGenerateToken_Expiration(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	claims := &JWTClaims{}
	parsedToken, _ := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtConfig.Secret), nil
	})

	assert.True(t, parsedToken.Valid)
	assert.NotNil(t, claims.ExpiresAt)
	assert.True(t, claims.ExpiresAt.After(time.Now()))
}

// Test different subscription tiers
func TestGenerateToken_SubscriptionTiers(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	tiers := []string{"free", "pro", "enterprise"}

	for _, tier := range tiers {
		token, err := jwtConfig.GenerateToken("user-123", "test@example.com", tier)
		assert.NoError(t, err)

		claims, err := jwtConfig.ValidateToken(token)
		assert.NoError(t, err)
		assert.Equal(t, tier, claims.Tier)
	}
}

// Test empty user ID
func TestGenerateToken_EmptyUserID(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, err := jwtConfig.GenerateToken("", "test@example.com", "free")
	assert.NoError(t, err)

	claims, err := jwtConfig.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "", claims.UserID)
}

// Test empty email
func TestGenerateToken_EmptyEmail(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, err := jwtConfig.GenerateToken("user-123", "", "free")
	assert.NoError(t, err)

	claims, err := jwtConfig.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "", claims.Email)
}

// Test signing method
func TestJWT_SigningMethod(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	parsedToken, _ := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtConfig.Secret), nil
	})

	assert.Equal(t, jwt.SigningMethodHS256, parsedToken.Method)
}

// Test issuer claim
func TestJWT_Issuer(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	claims := &JWTClaims{}
	parsedToken, _ := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtConfig.Secret), nil
	})

	assert.True(t, parsedToken.Valid)
	assert.Equal(t, "vocalize", claims.Issuer)
}

// Test issued at claim
func TestJWT_IssuedAt(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	before := time.Now()
	token, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")
	after := time.Now()

	claims := &JWTClaims{}
	parsedToken, _ := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtConfig.Secret), nil
	})

	assert.True(t, parsedToken.Valid)
	assert.NotNil(t, claims.IssuedAt)
	assert.True(t, claims.IssuedAt.Time.After(before) || claims.IssuedAt.Time.Equal(before))
	assert.True(t, claims.IssuedAt.Time.Before(after) || claims.IssuedAt.Time.Equal(after))
}

// Test token refresh scenario
func TestJWT_RefreshScenario(t *testing.T) {
	jwtConfig := &JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	// Generate initial token
	token1, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")
	claims1, _ := jwtConfig.ValidateToken(token1)
	oldExpiryTime := claims1.ExpiresAt.Time

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Generate new token (refresh)
	token2, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")
	claims2, _ := jwtConfig.ValidateToken(token2)

	// New token should have later expiry
	assert.True(t, claims2.ExpiresAt.Time.After(oldExpiryTime) || claims2.ExpiresAt.Time.Equal(oldExpiryTime))
}
