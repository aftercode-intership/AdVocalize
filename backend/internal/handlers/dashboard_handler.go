// backend/internal/handlers/dashboard_handler.go
package handlers

import (
	"context"
	"net/http"
	"time"

	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
)

type DashboardHandler struct {
	dashboardService *services.DashboardService
}

func NewDashboardHandler(ds *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: ds}
}

// GetStats handles GET /api/dashboard/stats
// Returns overview numbers for the top cards.
func (h *DashboardHandler) GetStats(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	stats, err := h.dashboardService.GetStats(ctx, userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load stats"})
	}

	return c.Status(http.StatusOK).JSON(stats)
}

// GetRecent handles GET /api/dashboard/recent
// Returns the user's most recent generated ads (script + audio if available).
func (h *DashboardHandler) GetRecent(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	ads, err := h.dashboardService.GetRecentAds(ctx, userID, 12)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load recent ads"})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"ads": ads})
}

// LinkAudio handles PATCH /api/dashboard/ads/:id/audio
// Called by the frontend after a TTS job completes to permanently
// save the audio URL against the script record.
func (h *DashboardHandler) LinkAudio(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
	}

	adID := c.Params("id")
	if adID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Ad ID required"})
	}

	var req struct {
		AudioURL        string  `json:"audio_url"`
		AudioJobID      string  `json:"audio_job_id"`
		AudioVoiceID    string  `json:"audio_voice_id"`
		AudioDurationS  float64 `json:"audio_duration_s"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.dashboardService.LinkAudio(ctx, adID, userID, services.AudioLinkRequest{
		AudioURL:       req.AudioURL,
		AudioJobID:     req.AudioJobID,
		AudioVoiceID:   req.AudioVoiceID,
		AudioDurationS: req.AudioDurationS,
	}); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Audio linked"})
}