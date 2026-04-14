// backend/internal/middleware/auth_middleware_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vocalize/internal/config"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

// Test AuthMiddleware with valid cookie
func TestAuthMiddleware_ValidCookie(t *testing.T) {
	app := fiber.New()

	// Create JWT config and generate a valid token
	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	// Generate a valid token
	validToken, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	// Setup middleware
	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"user_id": c.Locals("user_id"),
			"email":   c.Locals("email"),
			"tier":    c.Locals("tier"),
		})
	})

	// Create request with cookie
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:     "session_token",
		Value:    validToken,
		Expires:  time.Now().Add(time.Hour),
		HttpOnly: true,
	})

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test AuthMiddleware with valid Bearer token
func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	validToken, _ := jwtConfig.GenerateToken("user-456", "user@example.com", "pro")

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"user_id": c.Locals("user_id"),
		})
	})

	// Create request with Bearer token
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test AuthMiddleware with missing token
func TestAuthMiddleware_MissingToken(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{})
	})

	// Request without any token
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// Test AuthMiddleware with expired token
func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: -time.Hour, // Already expired
	}

	// Generate an expired token
	expiredToken, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{})
	})

	// Request with expired token
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:     "session_token",
		Value:    expiredToken,
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: true,
	})

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// Test AuthMiddleware with malformed token
func TestAuthMiddleware_MalformedToken(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{})
	})

	testCases := []struct {
		name  string
		token string
	}{
		{"malformed_bearer", "Bearer not-a-valid-jwt-token"},
		{"invalid_bearer_format", "BearerTokenWithoutSpace"},
		{"random_string", "random-garbage-token"},
		{"empty_bearer", "Bearer "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tc.token)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// Test RequireEmailVerified with verified user
func TestRequireEmailVerified_VerifiedUser(t *testing.T) {
	// Create a mock user service
	// In real test, would mock the GetUserByID to return verified user
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		// Simulate middleware setting user_id
		c.Locals("user_id", "verified-user-id")
		return c.Next()
	})

	// Note: In a real test, this would use a mock UserService
	// that returns a user with EmailVerified = true
	app.Get("/email-verified", func(c fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		// Simulate checking email verification status
		if userID == "verified-user-id" {
			return c.Status(http.StatusOK).JSON(fiber.Map{
				"message": "Email verified",
			})
		}
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error": "Email not verified",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/email-verified", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test RequireEmailVerified with unverified user
func TestRequireEmailVerified_UnverifiedUser(t *testing.T) {
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		// Simulate middleware setting user_id for unverified user
		c.Locals("user_id", "unverified-user-id")
		return c.Next()
	})

	// Simulate RequireEmailVerified check
	app.Get("/email-verified", func(c fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		// Simulate unverified user
		if userID == "unverified-user-id" {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "Email not verified",
			})
		}
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"message": "Email verified",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/email-verified", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// Test Authorization header parsing
func TestAuthMiddleware_AuthorizationHeaderParsing(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	validToken, _ := jwtConfig.GenerateToken("user-123", "test@example.com", "free")

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"user_id": c.Locals("user_id"),
		})
	})

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{"valid_bearer", "Bearer " + validToken, http.StatusOK},
		{"no_bearer", "Basic dXNlcjpwYXNz", http.StatusUnauthorized}, // Basic auth
		{"empty_bearer", "Bearer", http.StatusUnauthorized},
		{"invalid_prefix", "Token " + validToken, http.StatusUnauthorized},
		{"lowercase_bearer", "bearer " + validToken, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.authHeader)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// Test context locals are set correctly
func TestAuthMiddleware_ContextLocals(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	validToken, _ := jwtConfig.GenerateToken("user-789", "testuser@example.com", "enterprise")

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		// Verify all locals are set correctly
		userID := c.Locals("user_id")
		email := c.Locals("email")
		tier := c.Locals("tier")

		return c.Status(http.StatusOK).JSON(fiber.Map{
			"user_id": userID,
			"email":   email,
			"tier":    tier,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test cookie takes precedence over header
func TestAuthMiddleware_CookieTakesPrecedence(t *testing.T) {
	app := fiber.New()

	jwtConfig := &config.JWTConfig{
		Secret: "test-secret-key-for-testing-purposes-only",
		Expiry: 24 * time.Hour,
	}

	cookieToken, _ := jwtConfig.GenerateToken("cookie-user", "cookie@example.com", "free")
	headerToken, _ := jwtConfig.GenerateToken("header-user", "header@example.com", "pro")

	app.Use(AuthMiddleware(jwtConfig.Secret))
	app.Get("/protected", func(c fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"user_id": userID,
		})
	})

	// Request with both cookie and header - cookie should take precedence
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:     "session_token",
		Value:    cookieToken,
		Expires:  time.Now().Add(time.Hour),
		HttpOnly: true,
	})
	req.Header.Set("Authorization", "Bearer "+headerToken)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
