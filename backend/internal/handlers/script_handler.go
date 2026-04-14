// backend/internal/handlers/script_handler.go
package handlers

import (
	"context"
	"net/http"
	"time"

	"vocalize/internal/services"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// ScriptHandler handles POST /api/generate/script
// It validates the request, calls ScriptService, and returns the generated script.
type ScriptHandler struct {
	scriptService *services.ScriptService
	validator     *validator.Validate
}

func NewScriptHandler(scriptService *services.ScriptService) *ScriptHandler {
	return &ScriptHandler{
		scriptService: scriptService,
		validator:     validator.New(),
	}
}

// generateScriptRequest is the validated shape of the incoming JSON body.
// The `validate` tags are checked by go-playground/validator.
type generateScriptRequest struct {
	ProductName        string `json:"product_name"        validate:"required,min=2,max=100"`
	ProductDescription string `json:"product_description" validate:"required,min=10,max=1000"`
	TargetAudience     string `json:"target_audience"     validate:"required,min=5,max=255"`
	Tone               string `json:"tone"                validate:"required,oneof=FORMAL CASUAL PODCAST"`
	Language           string `json:"language"            validate:"required,oneof=en fr ar"`
	CampaignID         string `json:"campaign_id"`
	BrandGuidelines    string `json:"brand_guidelines"`
}

// GenerateScript handles POST /api/generate/script
//
// Request body (JSON):
//
//	{
//	  "product_name":        "Premium Headphones",
//	  "product_description": "Noise-canceling wireless headphones with 30h battery",
//	  "target_audience":     "Tech professionals aged 25-40",
//	  "tone":                "FORMAL",
//	  "language":            "en"
//	}
//
// Response (200 OK):
//
//	{
//	  "id":                         "uuid",
//	  "script_text":                "...",
//	  "language":                   "en",
//	  "tone":                       "FORMAL",
//	  "word_count":                 95,
//	  "estimated_duration_seconds": 32,
//	  "version":                    1,
//	  "status":                     "completed",
//	  "created_at":                 "2026-04-04T..."
//	}
func (h *ScriptHandler) GenerateScript(c fiber.Ctx) error {
	// Get authenticated user ID set by AuthMiddleware
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Parse and bind the request body
	var req generateScriptRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON body",
		})
	}

	// Validate struct tags (required, min, max, oneof)
	if err := h.validator.Struct(req); err != nil {
		// Return the first validation error in a user-friendly format
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": formatValidationError(err),
		})
	}

	// Call the service with a 30-second timeout
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	result, err := h.scriptService.Generate(ctx, userID, &services.ScriptGenerateRequest{
		ProductName:        req.ProductName,
		ProductDescription: req.ProductDescription,
		TargetAudience:     req.TargetAudience,
		Tone:               req.Tone,
		Language:           req.Language,
		CampaignID:         req.CampaignID,
		BrandGuidelines:    req.BrandGuidelines,
	})

	if err != nil {
		// Surface "insufficient credits" as 402 Payment Required
		if err.Error()[:21] == "insufficient credits:" {
			return c.Status(http.StatusPaymentRequired).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(result)
}

// GetScript handles GET /api/generate/script/:id
// Retrieves a previously generated script from the database.
func (h *ScriptHandler) GetScript(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	adID := c.Params("id")
	if adID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Script ID required",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	script, err := h.scriptService.GetGeneratedScript(ctx, adID, userID)
	if err != nil {
		if err.Error() == "script not found" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"error": "Script not found",
			})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve script",
		})
	}

	return c.Status(http.StatusOK).JSON(script)
}

// formatValidationError converts go-playground validator errors into a
// single readable string for the frontend.
func formatValidationError(err error) string {
	// validator.ValidationErrors is a slice — we return the first one
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			switch e.Tag() {
			case "required":
				return field + " is required"
			case "min":
				return field + " is too short (min " + e.Param() + " characters)"
			case "max":
				return field + " is too long (max " + e.Param() + " characters)"
			case "oneof":
				return field + " must be one of: " + e.Param()
			default:
				return field + " is invalid"
			}
		}
	}
	return "Invalid request"
}