// backend/internal/models/chat.go
package models

import (
  "time"
)

type ChatSession struct {
  ID         string    `json:"id"`
  UserID     string    `json:"user_id"`
  CampaignID string    `json:"campaign_id,omitempty"`
  AdID       string    `json:"ad_id,omitempty"`
  Topic      string    `json:"topic"`  // "script_refinement", "creative_ideas", "support"
  Status     string    `json:"status"` // active, archived, closed
  CreatedAt  time.Time `json:"created_at"`
  UpdatedAt  time.Time `json:"updated_at"`
}

type ChatMessage struct {
  ID        string    `json:"id"`
  SessionID string    `json:"session_id"`
  UserID    string    `json:"user_id"`
  Content   string    `json:"content"`
  Role      string    `json:"role"` // USER, ASSISTANT, SYSTEM
  IsEdited  bool      `json:"is_edited"`
  Version   int       `json:"version"`
  Metadata  *ChatMetadata `json:"metadata,omitempty"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}

type ChatMetadata struct {
  Type              string `json:"type"`              // "suggestion", "refinement", "escalation"
  SourceAdID        string `json:"source_ad_id,omitempty"`
  ContextScript     string `json:"context_script,omitempty"`
  ConfidenceScore   float64 `json:"confidence_score,omitempty"`
  EscalationReason  string `json:"escalation_reason,omitempty"`
  HumanAgentID      string `json:"human_agent_id,omitempty"`
}

type ChatRequest struct {
  SessionID string `json:"session_id"`
  Content   string `json:"content" validate:"required,min=1,max=2000"`
  Topic     string `json:"topic"`
  Context   *ChatContext `json:"context,omitempty"`
}

type ChatContext struct {
  CurrentScript     string `json:"current_script,omitempty"`
  PreviousMessages  []string `json:"previous_messages,omitempty"`
  Language          string `json:"language"`
  Tone              string `json:"tone"`
  TargetAudience    string `json:"target_audience,omitempty"`
}

type ChatResponse struct {
  ID        string       `json:"id"`
  Content   string       `json:"content"`
  Role      string       `json:"role"`
  Metadata  *ChatMetadata `json:"metadata,omitempty"`
  CreatedAt time.Time    `json:"created_at"`
}