package middleware

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"vocalize/internal/config"
)

func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Try to get token from cookie first
		token := c.Cookies("auth_token")

		// If not in cookie, try Authorization header
		if token == "" {
			authHeader := c.Get("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
		}

		if token == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization token",
			})
		}

		claims, err := config.VerifyToken(token, jwtSecret)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Store user ID in context
		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)

		return c.Next()
	}
}