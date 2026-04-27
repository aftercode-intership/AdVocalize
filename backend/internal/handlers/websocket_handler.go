// backend/internal/handlers/websocket_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	fws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"vocalize/internal/services"
)

var wsUpgrader = fws.FastHTTPUpgrader{
	CheckOrigin:      func(r *fasthttp.RequestCtx) bool { return true },
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

// WebSocketFiberHandler returns the fiber.Handler that upgrades HTTP → WebSocket.
// Registered in main.go as:
//
//	app.Get("/api/chat/sessions/:id/ws",
//	    middleware.AuthMiddleware(jwtSecret),
//	    h.Chat.WebSocketHandler(),
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

		return wsUpgrader.Upgrade(c.RequestCtx(), func(ws *fws.Conn) {
			defer ws.Close()
			log.Printf("[WS] user=%s session=%s connected", userID, sessionID)
			h.handleMessages(ws, sessionID, userID)
			log.Printf("[WS] user=%s session=%s disconnected", userID, sessionID)
		})
	}
}

// HandleWebSocket is the compatibility shim called from ChatHandler.WebSocketHandler().
func (h *WebSocketHandler) HandleWebSocket(ws *fws.Conn, sessionID, userID string) {
	h.handleMessages(ws, sessionID, userID)
}

func (h *WebSocketHandler) handleMessages(ws *fws.Conn, sessionID, userID string) {
	// Keep-alive: 60 s read deadline, refreshed on each pong
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping goroutine
	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ws.WriteControl(fws.PingMessage, []byte{}, time.Now().Add(5*time.Second))
			case <-done:
				return
			}
		}
	}()

	for {
		msgType, raw, err := ws.ReadMessage()
		if err != nil {
			if fws.IsUnexpectedCloseError(err,
				fws.CloseGoingAway,
				fws.CloseNormalClosure,
				fws.CloseNoStatusReceived,
			) {
				log.Printf("[WS] unexpected close session=%s: %v", sessionID, err)
			}
			return
		}
		if msgType != fws.TextMessage {
			continue
		}

		var incoming struct {
			Content string `json:"content"`
			Topic   string `json:"topic"`
		}
		if err := json.Unmarshal(raw, &incoming); err != nil {
			h.sendJSON(ws, map[string]string{"type": "error", "error": "Invalid message format"})
			continue
		}
		if incoming.Content == "" {
			h.sendJSON(ws, map[string]string{"type": "error", "error": "Content cannot be empty"})
			continue
		}
		if incoming.Topic == "" {
			incoming.Topic = "general"
		}

		// Typing indicator → send → stop
		h.sendJSON(ws, map[string]interface{}{"type": "typing", "is_typing": true})

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		result, err := h.chatService.SendMessage(ctx, &services.SendMessageRequest{
			SessionID: sessionID,
			UserID:    userID,
			Content:   incoming.Content,
			Topic:     incoming.Topic,
		})
		cancel() // always release context resources immediately

		h.sendJSON(ws, map[string]interface{}{"type": "typing", "is_typing": false})

		if err != nil {
			h.sendJSON(ws, map[string]string{
				"type":  "error",
				"error": "Failed to generate response: " + err.Error(),
			})
			continue
		}

		h.sendJSON(ws, map[string]interface{}{
			"type":         "message",
			"user_message": result.UserMessage,
			"ai_message":   result.AIMessage,
		})

		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

func (h *WebSocketHandler) sendJSON(ws *fws.Conn, v interface{}) {
	if b, err := json.Marshal(v); err == nil {
		ws.WriteMessage(fws.TextMessage, b)
	}
}

