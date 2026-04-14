package middleware

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"vocalize/internal/services"
)

type RBACMiddleware struct {
	userService *services.UserService
}

func NewRBACMiddleware(userService *services.UserService) *RBACMiddleware {
	return &RBACMiddleware{userService: userService}
}

// RequireRole checks if user has required role
func (rm *RBACMiddleware) RequireRole(requiredRole string) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(string)
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(map[string]string{
				"error": "Not authenticated",
			})
		}

		// For MVP, we only have two roles: user and admin
		// Admin check: user_id must be in admin list (could be from env or database)
		isAdmin, err := rm.userService.IsAdmin(userID)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(map[string]string{
				"error": "Failed to check permissions",
			})
		}

		if requiredRole == "admin" && !isAdmin {
			return c.Status(http.StatusForbidden).JSON(map[string]string{
				"error": "Admin privileges required",
			})
		}

		c.Locals("role", requiredRole)
		return c.Next()
	}
}
