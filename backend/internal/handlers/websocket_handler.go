// backend/internal/handlers/websocket_handler.go
// Uses github.com/fasthttp/websocket — compatible with Fiber v3's underlying
// fasthttp engine without needing the gofiber/contrib/websocket package.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	fws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
	"vocalize/internal/services"
)

// wsUpgrader upgrades HTTP → WebSocket using the fasthttp upgrader.
// CheckOrigin returns true to allow all origins in development.
// In production, validate against your allowed origins list.
var wsUpgrader = fws.FastHTTPUpgrader{
	CheckOrigin: func(r *fasthttp.RequestCtx) bool {
		return true // TODO: restrict in production
	},
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

type WebSocketHandler struct {
	chatService *services.ChatService
}

func NewWebSocketHandler(chatService *services.ChatService) *WebSocketHandler {
	return &WebSocketHandler{chatService: chatService}
}

// WebSocketFiberHandler returns a fiber.Handler that upgrades the connection.
// Used in chat_handler.go and registered in main.go as:
//
//	app.Get("/api/chat/sessions/:id/ws",
//	    middleware.AuthMiddleware(jwtSecret),
//	    wsHandler.WebSocketFiberHandler(),
//	)
func (h *WebSocketHandler) WebSocketFiberHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(string)
		if !ok || userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		sessionID := c.Params("id")
		if sessionID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Session ID required",
			})
		}

		// Upgrade the underlying fasthttp RequestCtx to WebSocket.
		// After this point, the connection is hijacked — we cannot write
		// to the fiber response anymore.
		err := wsUpgrader.Upgrade(c.RequestCtx(), func(ws *fws.Conn) {
			defer ws.Close()

			log.Printf("[WS] Connected: user=%s session=%s", userID, sessionID)
			h.handleMessages(ws, sessionID, userID)
			log.Printf("[WS] Disconnected: user=%s session=%s", userID, sessionID)
		})

		if err != nil {
			// Upgrade errors (e.g., client sent non-WS request) are not fatal
			log.Printf("[WS] Upgrade error for session %s: %v", sessionID, err)
			return err
		}

		return nil
	}
}

// handleMessages is the main read/write loop for an established WebSocket conn.
// Architecture: fetch-and-wait (not streaming) — the client sends a message,
// we call GLM synchronously, then push the reply back over the socket.
//
// This is intentionally simple. Streaming via token-by-token pushing will be
// added in a later sprint once the GLM streaming API is integrated.
func (h *WebSocketHandler) handleMessages(ws *fws.Conn, sessionID, userID string) {
	// Configure ping/pong to detect stale connections
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(appData string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping ticker — keeps the connection alive and detects dead clients
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Channel to signal the ping goroutine to stop when this func returns
	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			select {
			case <-pingTicker.C:
				if err := ws.WriteControl(
					fws.PingMessage,
					[]byte{},
					time.Now().Add(5*time.Second),
				); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		// Read the next message from the client
		messageType, rawMessage, err := ws.ReadMessage()
		if err != nil {
			if fws.IsUnexpectedCloseError(
				err,
				fws.CloseGoingAway,
				fws.CloseNormalClosure,
				fws.CloseNoStatusReceived,
			) {
				log.Printf("[WS] Unexpected close: %v", err)
			}
			return // connection closed — exit the loop
		}

		if messageType != fws.TextMessage {
			continue // ignore binary frames for now
		}

		// Parse the incoming message
		var incoming struct {
			Content string `json:"content"`
			Topic   string `json:"topic"`
		}

		if err := json.Unmarshal(rawMessage, &incoming); err != nil {
			h.sendError(ws, "Invalid message format")
			continue
		}

		if incoming.Content == "" {
			h.sendError(ws, "Message content cannot be empty")
			continue
		}

		if incoming.Topic == "" {
			incoming.Topic = "general"
		}

		// Send a "typing..." indicator so the frontend can show the spinner
		h.sendTyping(ws, true)

		// Call the chat service synchronously — GLM typically replies in 1-3s
		result, err := h.chatService.SendMessage(
			// Context with 25s timeout; WS connections outlive normal HTTP requests
			newContextWithTimeout(25*time.Second),
			&services.SendMessageRequest{
				SessionID: sessionID,
				UserID:    userID,
				Content:   incoming.Content,
				Topic:     incoming.Topic,
			},
		)

		// Stop typing indicator
		h.sendTyping(ws, false)

		if err != nil {
			h.sendError(ws, fmt.Sprintf("Failed to generate response: %v", err))
			continue
		}

		// Send both messages back to the client as a single JSON object
		response := map[string]interface{}{
			"type":         "message",
			"user_message": result.UserMessage,
			"ai_message":   result.AIMessage,
		}

		if err := ws.WriteJSON(response); err != nil {
			log.Printf("[WS] Write error: %v", err)
			return
		}

		// Reset the read deadline after each successful exchange
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// HandleWebSocket is a compatibility shim — called by ChatHandler.WebSocketHandler()
// which previously used the contrib/websocket.Conn type.
// Now both use the same fasthttp/websocket.Conn type.
func (h *WebSocketHandler) HandleWebSocket(ws *fws.Conn, sessionID, userID string) {
	h.handleMessages(ws, sessionID, userID)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *WebSocketHandler) sendError(ws *fws.Conn, message string) {
	ws.WriteJSON(map[string]string{
		"type":  "error",
		"error": message,
	})
}

func (h *WebSocketHandler) sendTyping(ws *fws.Conn, isTyping bool) {
	ws.WriteJSON(map[string]interface{}{
		"type":      "typing",
		"is_typing": isTyping,
	})
}

// newContextWithTimeout creates a context.Context with a deadline.
// Placed here to avoid importing context in the caller just for this.
func newContextWithTimeout(d time.Duration) context.Context {
	ctx, _ := context.WithTimeout(context.Background(), d)
	return ctx
}