package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"vocalize/internal/models"
	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
)

type ChatHandler struct {
	chatService      *services.ChatService
	websocketHandler *WebSocketHandler
}

func NewChatHandler(
	chatService *services.ChatService,
	websocketHandler *WebSocketHandler,
) *ChatHandler {
	return &ChatHandler{
		chatService:      chatService,
		websocketHandler: websocketHandler,
	}
}

// CreateSession creates a new chat session
// POST /api/chat/sessions
func (h *ChatHandler) CreateSession(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req struct {
		Topic string `json:"topic"`
	}

	if err := c.Bind().Body(&req); err != nil {
		req.Topic = "general" // Default topic
	}

	session, err := h.chatService.CreateSession(userID, nil, req.Topic)

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create session: " + err.Error(),
		})
	}

	return c.Status(http.StatusCreated).JSON(session)
}

// ListSessions retrieves all sessions for the current user
// GET /api/chat/sessions
func (h *ChatHandler) ListSessions(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	sessions, err := h.chatService.ListSessions(userID, 20)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load sessions: " + err.Error(),
		})
	}

	// Ensure we never return null — an empty array is better for the frontend
	if sessions == nil {
		sessions = []models.ChatSession{}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"sessions": sessions,
	})
}

// GetHistory retrieves message history for a session
// GET /api/chat/sessions/:id/history
func (h *ChatHandler) GetHistory(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	sessionID := c.Params("id")
	if sessionID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Session ID required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify the user owns this session
	session, err := h.chatService.GetSessionByID(ctx, sessionID)
	if err != nil || session == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Session not found",
		})
	}

	if session.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get the messages
	messages, err := h.chatService.GetSessionHistory(sessionID, 50)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch history: " + err.Error(),
		})
	}

	if messages == nil {
		messages = []models.ChatMessage{}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"messages": messages,
		"count":    len(messages),
	})
}

// SendMessage sends a message and gets a response
// POST /api/chat/sessions/:id/message
func (h *ChatHandler) SendMessage(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	sessionID := c.Params("id")
	if sessionID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Session ID required",
		})
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Content == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Message content is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Verify user owns session
	session, err := h.chatService.GetSessionByID(ctx, sessionID)
	if err != nil || session == nil || session.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	result, err := h.chatService.SendMessage(ctx, &services.SendMessageRequest{
		SessionID: sessionID,
		UserID:    userID,
		Content:   req.Content,
		Topic:     session.Topic,
	})

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to send message: " + err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(result)
}

// ArchiveSession archives a chat session
// DELETE /api/chat/sessions/:id
func (h *ChatHandler) ArchiveSession(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	sessionID := c.Params("id")
	if sessionID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Session ID required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify user owns session
	session, err := h.chatService.GetSessionByID(ctx, sessionID)
	if err != nil || session == nil || session.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.chatService.ArchiveSession(sessionID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to archive session",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "Session archived",
	})
}

// EnhancePrompt enhances a user's prompt using AI
// POST /api/chat/enhance
func (h *ChatHandler) EnhancePrompt(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req struct {
		Prompt   string `json:"prompt"`
		Language string `json:"language"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.Prompt) < 5 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Prompt must be at least 5 characters",
		})
	}

	if req.Language == "" {
		req.Language = "en"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	enhanced, err := h.chatService.EnhancePrompt(ctx, req.Prompt, req.Language)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Enhancement failed: " + err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"enhanced_prompt": enhanced,
	})
}

// GetLibraryHistory retrieves the user's script library history
// GET /api/library/history
// THIS WAS MISSING! Now it's implemented.
func (h *ChatHandler) GetLibraryHistory(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Get pagination params
	limitStr := c.Query("limit", "10")
	offsetStr := c.Query("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// For now, return empty history since script library is in Sprint 3
	// This is a placeholder that prevents 500 errors
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"scripts": []interface{}{},
		"total":   0,
		"limit":   limit,
		"offset":  offset,
	})
}

// WebSocketHandler handles real-time chat via WebSocket
// GET /api/chat/sessions/:id/ws
func (h *ChatHandler) WebSocketHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		return h.websocketHandler.WebSocketFiberHandler()(c)
	}
}
