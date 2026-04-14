// backend/internal/services/support_bot_service.go
package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SupportBotService struct {
	chatService *ChatService
	db          *sql.DB
}

func NewSupportBotService(chatService *ChatService, db *sql.DB) *SupportBotService {
	return &SupportBotService{
		chatService: chatService,
		db:          db,
	}
}

type EscalationRequest struct {
	ID             string
	SessionID      string
	UserID         string
	Reason         string
	ConversationID string
	CreatedAt      time.Time
	AssignedTo     *string
}

// AnalyzeForEscalation analyzes message to determine if escalation needed
func (sbs *SupportBotService) AnalyzeForEscalation(message string) (bool, string) {
	// Keywords that suggest need for escalation
	escalationKeywords := []string{
		"bug", "error", "crash", "broken", "not working",
		"payment issue", "billing", "charge",
		"urgent", "critical", "help", "support",
		"speak to", "agent", "human",
	}

	for _, keyword := range escalationKeywords {
		if strings.Contains(strings.ToLower(message), keyword) {
			return true, keyword
		}
	}

	return false, ""
}

// CreateEscalation creates support escalation ticket
func (sbs *SupportBotService) CreateEscalation(
	sessionID string,
	userID string,
	reason string,
) (*EscalationRequest, error) {
	escalation := &EscalationRequest{
		ID:             uuid.New().String(),
		SessionID:      sessionID,
		UserID:         userID,
		Reason:         reason,
		ConversationID: sessionID,
		CreatedAt:      time.Now(),
	}

	query := `
    INSERT INTO support_escalations (id, session_id, user_id, reason, created_at, status)
    VALUES ($1, $2, $3, $4, $5, $6)
  `

	_, err := sbs.db.Exec(
		query,
		escalation.ID, escalation.SessionID, escalation.UserID, escalation.Reason, escalation.CreatedAt, "pending",
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create escalation: %w", err)
	}

	// Send notification to support team
	sbs.notifySupportTeam(escalation)

	return escalation, nil
}

// AssignToAgent assigns escalation to support agent
func (sbs *SupportBotService) AssignToAgent(escalationID string, agentID string) error {
	query := `
    UPDATE support_escalations
    SET assigned_to = $1, status = 'assigned', updated_at = CURRENT_TIMESTAMP
    WHERE id = $2
  `

	_, err := sbs.db.Exec(query, agentID, escalationID)
	return err
}

// CloseEscalation closes escalation ticket
func (sbs *SupportBotService) CloseEscalation(escalationID string, resolution string) error {
	query := `
    UPDATE support_escalations
    SET status = 'resolved', resolution = $1, resolved_at = CURRENT_TIMESTAMP
    WHERE id = $2
  `

	_, err := sbs.db.Exec(query, resolution, escalationID)
	return err
}

// GetPendingEscalations gets all pending escalations
func (sbs *SupportBotService) GetPendingEscalations() ([]EscalationRequest, error) {
	query := `
    SELECT id, session_id, user_id, reason, created_at
    FROM support_escalations
    WHERE status = 'pending'
    ORDER BY created_at ASC
  `

	rows, err := sbs.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var escalations []EscalationRequest
	for rows.Next() {
		var e EscalationRequest
		if err := rows.Scan(&e.ID, &e.SessionID, &e.UserID, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		escalations = append(escalations, e)
	}

	return escalations, nil
}

func (sbs *SupportBotService) notifySupportTeam(escalation *EscalationRequest) {
	// In production: Send email, Slack notification, or database alert
	fmt.Printf("Support team notified: escalation %s for user %s\n", escalation.ID, escalation.UserID)
}
