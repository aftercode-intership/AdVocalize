package handlers

import (
	"net/http"

	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
)

type MessageEditHandler struct {
	editService *services.MessageEditService
}

func NewMessageEditHandler(editService *services.MessageEditService) *MessageEditHandler {
	return &MessageEditHandler{editService: editService}
}

// EditMessage handles PATCH /api/chat/messages/:id
func (h *MessageEditHandler) EditMessage(c fiber.Ctx) error {

	messageID := c.Params("id")

	var req struct {
		Content string `json:"content" validate:"required,min=1,max=2000"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request",
		})
	}

	if err := h.editService.EditMessage(messageID, req.Content); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to edit message",
		})
	}

	return c.Status(http.StatusOK).JSON(map[string]string{
		"message": "Message updated",
	})
}

// RegenerateResponse handles POST /api/chat/messages/:id/regenerate
func (h *MessageEditHandler) RegenerateResponse(c fiber.Ctx) error {

	messageID := c.Params("id")

	var req struct {
		SessionID string `json:"session_id" validate:"required"`
		Topic     string `json:"topic"`
		Context   *services.PromptContext `json:"context"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request",
		})
	}

	newMessage, err := h.editService.RegenerateResponse(
		messageID,
		req.SessionID,
		nil, // Convert to models.ChatContext
		req.Topic,
	)

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to regenerate response",
		})
	}

	return c.Status(http.StatusOK).JSON(newMessage)
}

// GetVersions handles GET /api/chat/messages/:id/versions
func (h *MessageEditHandler) GetVersions(c fiber.Ctx) error {
	messageID := c.Params("id")

	versions, err := h.editService.GetMessageVersions(messageID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to get versions",
		})
	}

	return c.Status(http.StatusOK).JSON(map[string]interface{}{
		"versions": versions,
	})
}
