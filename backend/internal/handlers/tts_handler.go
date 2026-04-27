// backend/internal/handlers/tts_handler.go
package handlers

import (
	"context"
	"net/http"
	"time"

	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
)

type TTSHandler struct {
	ttsService *services.TTSService
}

func NewTTSHandler(ttsService *services.TTSService) *TTSHandler {
	return &TTSHandler{ttsService: ttsService}
}

// GenerateAudio handles POST /api/generate/audio
//
// Request:
//
//	{
//	  "script_text": "...",
//	  "language":    "en",
//	  "voice_id":    "en-US-AriaNeural",  // optional
//	  "speed":       1.0,                  // optional, default 1.0
//	  "ad_id":       "uuid"               // optional, links audio to a script
//	}
//
// Response (200 — job enqueued):
//
//	{ "job_id": "uuid", "status": "queued", "message": "..." }
func (h *TTSHandler) GenerateAudio(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req services.TTSGenerateRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.ScriptText == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "script_text is required"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	result, err := h.ttsService.Generate(ctx, userID, &req)
	if err != nil {
		if len(err.Error()) > 21 && err.Error()[:21] == "insufficient credits:" {
			return c.Status(http.StatusPaymentRequired).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(result)
}

// GetAudioStatus handles GET /api/generate/status/:jobId
//
// Response examples:
//
//	{ "job_id": "...", "status": "queued" }
//	{ "job_id": "...", "status": "processing" }
//	{ "job_id": "...", "status": "completed", "audio_url": "https://...", "duration_seconds": 28.4 }
//	{ "job_id": "...", "status": "failed", "error": "edge-tts returned no audio" }
func (h *TTSHandler) GetAudioStatus(c fiber.Ctx) error {
	_, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
	}

	jobID := c.Params("jobId")
	if jobID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Job ID required"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	status, err := h.ttsService.GetStatus(ctx, jobID)
	if err != nil {
		if err.Error() == "job not found" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Job not found"})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get job status"})
	}

	return c.Status(http.StatusOK).JSON(status)
}

// GetVoices handles GET /api/generate/voices
// Returns available TTS voices grouped by language.
func (h *TTSHandler) GetVoices(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	voices, err := h.ttsService.GetVoices(ctx)
	if err != nil {
		// Return fallback voices if TTS service is down
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"voices": fiber.Map{
				"en": []fiber.Map{{"id": "en-US-AriaNeural", "name": "Aria (US, Female)", "gender": "Female"}},
				"fr": []fiber.Map{{"id": "fr-FR-DeniseNeural", "name": "Denise (French, Female)", "gender": "Female"}},
				"ar": []fiber.Map{{"id": "ar-SA-ZariyahNeural", "name": "Zariyah (Saudi, Female)", "gender": "Female"}},
			},
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"voices": voices})
}