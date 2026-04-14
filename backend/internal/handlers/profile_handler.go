// backend/internal/handlers/profile_handler.go
package handlers

import (
	"fmt"
	"net/http"

	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
)

type ProfileUpdateRequest struct {
	Name     string `json:"name"`
	Company  string `json:"company"`
	Bio      string `json:"bio"`
	Language string `json:"language"` // en, fr, ar
}

type ProfileResponse struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Name      string  `json:"name"`
	Company   *string `json:"company"`
	Bio       *string `json:"bio"`
	Language  string  `json:"language"`
	AvatarURL *string `json:"avatar_url"`
	Tier      string  `json:"tier"`
}

type ProfileHandler struct {
	profileService *services.ProfileService
}

func NewProfileHandler(
	profileService *services.ProfileService,
) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

// GetProfile handles GET /api/users/profile
func (h *ProfileHandler) GetProfile(c fiber.Ctx) error {

	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	user, err := h.profileService.GetProfile(userID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(map[string]string{
			"error": "User not found",
		})
	}

	return c.Status(http.StatusOK).JSON(ProfileResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Company:   user.Company,
		Bio:       user.Bio,
		Language:  user.Language,
		AvatarURL: user.Avatar,
		Tier:      user.SubscriptionTier,
	})
}

// UpdateProfile handles PATCH /api/users/profile
func (h *ProfileHandler) UpdateProfile(c fiber.Ctx) error {

	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	var req ProfileUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request body",
		})
	}

	// Validate language
	if req.Language != "" && !isValidLanguage(req.Language) {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid language. Must be: en, fr, ar",
		})
	}

	user, err := h.profileService.UpdateProfile(userID, req.Name, req.Company, req.Bio, req.Language)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to update profile",
		})
	}

	return c.Status(http.StatusOK).JSON(ProfileResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Company:   user.Company,
		Bio:       user.Bio,
		Language:  user.Language,
		AvatarURL: user.Avatar,
		Tier:      user.SubscriptionTier,
	})
}

// UploadAvatar handles POST /api/users/avatar (TODO: implement S3 upload)

// ExportData handles GET /api/users/export (GDPR)
func (h *ProfileHandler) ExportData(c fiber.Ctx) error {

	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	data, err := h.profileService.ExportUserData(userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to export data",
		})
	}

	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=vocalize-export-%s.json", userID))
	return c.Send(data)
}

func isValidLanguage(lang string) bool {
	return lang == "en" || lang == "fr" || lang == "ar"
}
