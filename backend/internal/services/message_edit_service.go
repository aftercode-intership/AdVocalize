// backend/internal/services/message_edit_service.go
package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"vocalize/internal/models"

	"github.com/google/uuid"
)

type MessageEditService struct {
	db          *sql.DB
	chatService *ChatService
}

func NewMessageEditService(db *sql.DB, chatService *ChatService) *MessageEditService {
	return &MessageEditService{
		db:          db,
		chatService: chatService,
	}
}

// EditMessage updates a message and creates version history
func (mes *MessageEditService) EditMessage(
	messageID string,
	newContent string,
) error {
	tx, err := mes.db.Begin()
	if err != nil {
		return fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback()

	// Get current message
	var currentContent, sessionID string
	var version int

	err = tx.QueryRow(`
    SELECT content, session_id, version
    FROM chat_messages
    WHERE id = $1
  `, messageID).Scan(&currentContent, &sessionID, &version)

	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}

	// Save current version to history
	_, err = tx.Exec(`
    INSERT INTO chat_message_versions (id, message_id, content, version, created_at)
    VALUES ($1, $2, $3, $4, $5)
  `, uuid.New().String(), messageID, currentContent, version, time.Now())

	if err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	// Update message with new content
	_, err = tx.Exec(`
    UPDATE chat_messages
    SET content = $1, is_edited = TRUE, version = version + 1, updated_at = CURRENT_TIMESTAMP
    WHERE id = $2
  `, newContent, messageID)

	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return tx.Commit()
}

// RegenerateResponse regenerates AI response for a user message
func (mes *MessageEditService) RegenerateResponse(
	userMessageID string,
	sessionID string,
	chatContext *models.ChatContext,
	topic string,
) (*models.ChatMessage, error) {
	// Get original user message
	var userContent string
	err := mes.db.QueryRow(`
    SELECT content FROM chat_messages WHERE id = $1 AND role = 'USER'
  `, userMessageID).Scan(&userContent)

	if err != nil {
		return nil, fmt.Errorf("user message not found: %w", err)
	}

	// Generate new response
	newResponse, err := mes.chatService.GenerateContextualResponse(
		context.Background(),
		userContent,
		sessionID,
		chatContext,
		topic,
	)

	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Find and delete old assistant response following this user message
	var oldResponseID string
	err = mes.db.QueryRow(`
    SELECT id FROM chat_messages
    WHERE session_id = $1 AND created_at > (
      SELECT created_at FROM chat_messages WHERE id = $2
    ) AND role = 'ASSISTANT'
    ORDER BY created_at ASC
    LIMIT 1
  `, sessionID, userMessageID).Scan(&oldResponseID)

	// If old response exists, mark it
	if err == nil {
		mes.db.Exec(`UPDATE chat_messages SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1`, oldResponseID)
	}

	// Create new response message
	newMessage := &models.ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		UserID:    "",
		Content:   newResponse,
		Role:      "ASSISTANT",
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = mes.chatService.SaveMessage(newMessage)
	if err != nil {
		return nil, err
	}

	return newMessage, nil
}

// GetMessageVersions retrieves all versions of a message
func (mes *MessageEditService) GetMessageVersions(messageID string) ([]map[string]interface{}, error) {
	query := `
    SELECT id, content, version, created_at
    FROM chat_message_versions
    WHERE message_id = $1
    ORDER BY version ASC
  `

	rows, err := mes.db.Query(query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []map[string]interface{}
	for rows.Next() {
		var id, content string
		var version int
		var createdAt time.Time

		if err := rows.Scan(&id, &content, &version, &createdAt); err != nil {
			return nil, err
		}

		versions = append(versions, map[string]interface{}{
			"id":        id,
			"content":   content,
			"version":   version,
			"createdAt": createdAt,
		})
	}

	return versions, nil
}
