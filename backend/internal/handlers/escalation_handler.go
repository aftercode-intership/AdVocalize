package handlers

import (
	"vocalize/internal/services"

	"net/http"

	"github.com/gofiber/fiber/v3"
)

type EscalationHandler struct {
	supportBot *services.SupportBotService
}

func NewEscalationHandler(supportBot *services.SupportBotService) *EscalationHandler {
	return &EscalationHandler{supportBot: supportBot}
}

// CreateEscalation handles POST /api/support/escalate
func (h *EscalationHandler) CreateEscalation(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	var req struct {
		SessionID string `json:"session_id" validate:"required"`
		Reason    string `json:"reason" validate:"required"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request",
		})
	}

	escalation, err := h.supportBot.CreateEscalation(req.SessionID, userID, req.Reason)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to create escalation",
		})
	}

	return c.Status(http.StatusCreated).JSON(escalation)
}

// GetPendingEscalations handles GET /api/admin/support/escalations (admin only)
func (h *EscalationHandler) GetPendingEscalations(c fiber.Ctx) error {
	escalations, err := h.supportBot.GetPendingEscalations()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to fetch escalations",
		})
	}

	return c.Status(http.StatusOK).JSON(map[string]interface{}{
		"escalations": escalations,
		"count":       len(escalations),
	})
}

// AssignEscalation handles POST /api/admin/support/escalations/:id/assign (admin only)
func (h *EscalationHandler) AssignEscalation(c fiber.Ctx) error {
	escalationID := c.Params("id")

	var req struct {
		AgentID string `json:"agent_id" validate:"required"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request",
		})
	}

	if err := h.supportBot.AssignToAgent(escalationID, req.AgentID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to assign escalation",
		})
	}

	return c.Status(http.StatusOK).JSON(map[string]string{
		"message": "Escalation assigned",
	})
}

// CloseEscalation handles POST /api/admin/support/escalations/:id/close (admin only)
func (h *EscalationHandler) CloseEscalation(c fiber.Ctx) error {
	escalationID := c.Params("id")

	var req struct {
		Resolution string `json:"resolution"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request",
		})
	}

	if err := h.supportBot.CloseEscalation(escalationID, req.Resolution); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to close escalation",
		})
	}

	return c.Status(http.StatusOK).JSON(map[string]string{
		"message": "Escalation closed",
	})
}
