package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"vocalize/internal/models"

	"github.com/google/uuid"
)

type ChatService struct {
	db            *sql.DB
	glmService    *GLMService // For AI responses
	creditService *CreditService
	pcm           *PromptContextManager // Add for context-aware responses
}

func NewChatService(
	db *sql.DB,
	glmService *GLMService,
	creditService *CreditService,
	pcm *PromptContextManager,
) *ChatService {
	return &ChatService{
		db:            db,
		glmService:    glmService,
		creditService: creditService,
		pcm:           pcm,
	}
}

// CreateSession creates a new chat session
func (cs *ChatService) CreateSession(
	userID string,
	campaignID *string,
	topic string,
) (*models.ChatSession, error) {
	session := &models.ChatSession{
		ID:         uuid.New().String(),
		UserID:     userID,
		CampaignID: "",
		Topic:      topic,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if campaignID != nil {
		session.CampaignID = *campaignID
	}

	query := `
    INSERT INTO chat_sessions (id, user_id, campaign_id, topic, status, created_at, updated_at)
    VALUES ($1, $2, NULLIF(TRIM($3), '')::uuid, $4, $5, $6, $7)
  `

	_, err := cs.db.Exec(
		query,
		session.ID, session.UserID, campaignID, session.Topic, session.Status, session.CreatedAt, session.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("Chat session created: id=%s, user=%s, topic=%s", session.ID, userID, topic)
	return session, nil
}

func (cs *ChatService) ListSessions(userID string, limit int) ([]models.ChatSession, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	query := `
		SELECT id, user_id, COALESCE(campaign_id::text, '') as campaign_id, COALESCE(ad_id::text, '') as ad_id,
		       topic, status, created_at, updated_at
		FROM chat_sessions
		WHERE user_id = $1::uuid
		  AND (status != 'archived' OR status IS NULL)
		ORDER BY updated_at DESC NULLS LAST
		LIMIT $2
	`

	rows, err := cs.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.ChatSession
	for rows.Next() {
		var s models.ChatSession
		err := rows.Scan(
			&s.ID, &s.UserID, &s.CampaignID, &s.AdID,
			&s.Topic, &s.Status, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// SaveMessage saves a chat message
func (cs *ChatService) SaveMessage(message *models.ChatMessage) error {
	// ── CRITICAL FIX: Use NULLIF to convert empty user_id to NULL for AI messages ──
	query := `
    INSERT INTO chat_messages (id, session_id, user_id, content, role, is_edited, version, metadata, created_at, updated_at)
    VALUES ($1, $2, NULLIF(TRIM($3), '')::uuid, $4, $5, $6, $7, $8, $9, $10)
    ON CONFLICT (id) DO UPDATE SET
      is_edited = EXCLUDED.is_edited,
      version = EXCLUDED.version,
      content = EXCLUDED.content,
      updated_at = EXCLUDED.updated_at
  `

	metadataBytes, _ := json.Marshal(message.Metadata)
	metadataJSON := string(metadataBytes)
	if metadataJSON == "null" {
		metadataJSON = "{}"
	}

	_, err := cs.db.Exec(
		query,
		message.ID, message.SessionID, message.UserID, message.Content, message.Role,
		message.IsEdited, message.Version, metadataJSON, message.CreatedAt, message.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	return nil
}

type SendMessageRequest struct {
	SessionID   string
	UserID      string
	Content     string
	Topic       string
	Context     *models.ChatContext // optional: enriches the AI prompt
	Attachments []MessageAttachment // optional: uploaded files
}

// MessageAttachment describes a file the user attached to their message.
type MessageAttachment struct {
	FileURL  string // MinIO URL (or data URL for small images)
	FileType string // "image", "audio", "document"
	FileName string
	MimeType string
	// For images: base64 content for sending to GLM-4V when it's supported
	// For now we extract a text description and include it in the prompt
	TextContent string // extracted text content (for documents) or description
}

// SendMessageResponse is what the HTTP handler returns to the frontend.
type SendMessageResponse struct {
	UserMessage *models.ChatMessage `json:"user_message"`
	AIMessage   *models.ChatMessage `json:"ai_message"`
}

// text responses from GLM typically arrive in 1-3 seconds.
func (cs *ChatService) SendMessage(ctx context.Context, req *SendMessageRequest) (*SendMessageResponse, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("request context cancelled")
	}
	now := time.Now()

	// 1. Build the user message, appending attachment descriptions to the content
	//    so the AI knows what files were shared
	enrichedContent := req.Content
	if len(req.Attachments) > 0 {
		enrichedContent += cs.buildAttachmentContext(req.Attachments)
	}

	userMsg := &models.ChatMessage{
		ID:        uuid.New().String(),
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Content:   req.Content, // save original content (not enriched) for display
		Role:      "USER",
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := cs.SaveMessage(userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// 2. Touch the session's updated_at so it bubbles to the top of the list
	cs.db.Exec(
		`UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1`,
		req.SessionID,
	)

	// 3. Fetch recent message history for context
	//    We send the last 5 messages so the AI remembers what was discussed
	//    (reduced from 10 because NVIDIA times out on very long prompts)
	history, err := cs.GetSessionHistory(req.SessionID, 5)
	if err != nil {
		// Non-fatal: we can still generate a response without history
		history = []models.ChatMessage{}
	}

	// 4. Build a context-enriched prompt
	//    The enrichedContent includes attachment descriptions; history gives memory
	aiContent, err := cs.generateWithHistory(ctx, enrichedContent, history, req.Topic, req.Context)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// 5. Save the AI response
	aiMsg := &models.ChatMessage{
		ID:        uuid.New().String(),
		SessionID: req.SessionID,
		UserID:    "", // empty = AI / system — saved as NULL via NULLIF in SaveMessage
		Content:   aiContent,
		Role:      "ASSISTANT",
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := cs.SaveMessage(aiMsg); err != nil {
		// Non-fatal: return the message even if we couldn't save it
		fmt.Printf("Warning: failed to save AI message: %v\n", err)
	}

	return &SendMessageResponse{
		UserMessage: userMsg,
		AIMessage:   aiMsg,
	}, nil
}

// GetSessionHistory retrieves chat history
func (cs *ChatService) GetSessionHistory(sessionID string, limit int) ([]models.ChatMessage, error) {
	query := `
    SELECT id, session_id, user_id, content, role, is_edited, version, created_at, updated_at
    FROM chat_messages
    WHERE session_id = $1
    ORDER BY created_at ASC
    LIMIT $2
  `

	rows, err := cs.db.Query(query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []models.ChatMessage
	for rows.Next() {
		var msg models.ChatMessage
		err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.UserID, &msg.Content, &msg.Role,
			&msg.IsEdited, &msg.Version, &msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
func (cs *ChatService) GetLibraryHistory(userID string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	query := `
		SELECT id, product_name, language, tone,
		       LEFT(script_text, 100) as script_preview,
		       created_at
		FROM generated_ads
		WHERE user_id = $1
		  AND status = 'completed'
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := cs.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query library: %w", err)
	}
	defer rows.Close()

	var ads []map[string]interface{}
	for rows.Next() {
		var id, productName, language, tone, scriptPreview string
		var createdAt time.Time

		if err := rows.Scan(&id, &productName, &language, &tone, &scriptPreview, &createdAt); err != nil {
			continue // skip malformed rows
		}

		ads = append(ads, map[string]interface{}{
			"id":           id,
			"product_name": productName,
			"language":     language,
			"tone":         tone,
			"script_text":  scriptPreview,
			"created_at":   createdAt,
		})
	}

	return ads, nil
}

// GenerateAIResponse generates AI response via GLM
func (cs *ChatService) GenerateAIResponse(
	ctx context.Context,
	userMessage string,
	context *models.ChatContext,
	topic string,
) (string, error) {
	// Check credits first
	balance, _ := cs.creditService.GetBalance(context.Language) // Placeholder
	if balance >= 0 && balance < 1 {
		return "", fmt.Errorf("insufficient credits")
	}

	// Build context-aware prompt
	prompt := cs.buildChatPrompt(userMessage, context, topic)

	// Call GLM with context
	response, err := cs.glmService.GenerateResponse(ctx, prompt, context.Language)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	return response, nil
}

// GetSessionTopic returns session topic if user owns it
func (cs *ChatService) GetSessionTopic(userID, sessionID string) (string, bool, error) {
	query := `
		SELECT topic FROM chat_sessions 
		WHERE id = $1 AND user_id = $2 AND status = 'active'
	`
	var topic string
	err := cs.db.QueryRow(query, sessionID, userID).Scan(&topic)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return topic, true, nil
}

// GenerateContextualAIResponse for websocket - fetches history + builds context
func (cs *ChatService) GenerateContextualAIResponse(sessionID, userID, userContent string) (string, error) {
	// Get session topic
	topic, ok, err := cs.GetSessionTopic(userID, sessionID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("session not found or inactive")
	}

	// Get recent history
	history, err := cs.GetSessionHistory(sessionID, 6)
	if err != nil {
		return "", err
	}

	// Build previous messages
	var previous []string
	for _, msg := range history {
		role := "User"
		if msg.Role == "ASSISTANT" {
			role = "Assistant"
		}
		previous = append(previous, fmt.Sprintf("%s: %s", role, msg.Content))
	}

	// Minimal context from PCM type
	promptCtx := &PromptContext{
		Topic:    topic,
		Language: "en", // default or from user profile
	}

	// Use PCM to build enriched prompt
	enrichedPrompt := cs.pcm.BuildEnrichedPrompt(userContent, promptCtx, previous)

	// Generate with background context (WebSocket doesn't have HTTP request context)
	response, err := cs.glmService.GenerateResponse(context.Background(), enrichedPrompt, promptCtx.Language)
	if err != nil {
		return "", err
	}

	return response, nil
}

// GetSessionByID retrieves a chat session by ID
func (cs *ChatService) GetSessionByID(ctx context.Context, sessionID string) (*models.ChatSession, error) {
	session := &models.ChatSession{}
	query := `
		SELECT id, user_id, COALESCE(campaign_id::text, '') as campaign_id, COALESCE(ad_id::text, '') as ad_id, topic, status, created_at, updated_at
		FROM chat_sessions WHERE id = $1
	`
	err := cs.db.QueryRowContext(ctx, query, sessionID).Scan(&session.ID, &session.UserID, &session.CampaignID, &session.AdID, &session.Topic, &session.Status, &session.CreatedAt, &session.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return session, nil
}

func (cs *ChatService) generateWithHistory(
	ctx context.Context,
	userMessage string,
	history []models.ChatMessage,
	topic string,
	chatCtx *models.ChatContext,
) (string, error) {
	// Build the system role description based on topic
	systemRole := cs.getSystemRole(topic)

	// Format message history as a readable transcript
	// GLM understands "User: ... \nAssistant: ..." style formatting
	var historyText string
	if len(history) > 0 {
		historyText = "\n\n=== CONVERSATION HISTORY ===\n"
		for _, msg := range history {
			prefix := "User"
			if msg.Role == "ASSISTANT" {
				prefix = "Assistant"
			}
			historyText += fmt.Sprintf("%s: %s\n", prefix, msg.Content)
		}
		historyText += "=== END HISTORY ===\n"
	}

	// Include any script/generation context the user opened the chat from
	var contextText string
	if chatCtx != nil && chatCtx.CurrentScript != "" {
		contextText = fmt.Sprintf("\n\n=== CURRENT SCRIPT BEING DISCUSSED ===\n%s\n=== END SCRIPT ===\n", chatCtx.CurrentScript)
	}

	fullPrompt := fmt.Sprintf(
		"%s%s%s\n\nUser: %s",
		systemRole, contextText, historyText, userMessage,
	)

	language := "en"
	if chatCtx != nil && chatCtx.Language != "" {
		language = chatCtx.Language
	}

	// Pass context for cancellation support
	return cs.glmService.GenerateResponse(ctx, fullPrompt, language)
}

// getSystemRole returns the AI persona description for each topic type.
func (cs *ChatService) getSystemRole(topic string) string {
	switch topic {
	case "script_refinement":
		return `You are an expert copywriter and marketing strategist. 
Help the user refine and improve their ad scripts. Be specific, give concrete examples, 
and explain the reasoning behind your suggestions. Focus on the Hook-Problem-Solution-CTA structure.`

	case "creative_ideas":
		return `You are a creative marketing strategist with expertise in audio and video advertising.
Generate original, compelling ideas for campaigns. Consider the target audience, 
emotional resonance, and practical production requirements.`

	case "support":
		return `You are a helpful support specialist for the Vocalize platform. 
Answer questions clearly, provide step-by-step guidance, and suggest workarounds when needed.`

	case "general":
		return `You are a helpful AI assistant for Vocalize, an AI-powered audio advertisement platform.
Help users generate scripts, improve their marketing copy, and create compelling ad content.
You can discuss script refinement, creative directions, audio production concepts, and general marketing strategy.`

	default:
		return `You are a helpful AI assistant for the Vocalize audio advertisement platform.`
	}
}

// buildAttachmentContext converts file attachments into text context for the AI.
// Since GLM-4-flash doesn't support vision directly, we describe files in text.
// When GLM-4V integration is added (a later sprint), images can be sent directly.
func (cs *ChatService) buildAttachmentContext(attachments []MessageAttachment) string {
	if len(attachments) == 0 {
		return ""
	}

	context := "\n\n[User also attached the following files:]\n"
	for _, a := range attachments {
		switch a.FileType {
		case "image":
			context += fmt.Sprintf("- Image file: %s\n", a.FileName)
			if a.TextContent != "" {
				context += fmt.Sprintf("  Description: %s\n", a.TextContent)
			}
		case "audio":
			context += fmt.Sprintf("- Audio recording: %s (will be processed by TTS pipeline)\n", a.FileName)
		case "document":
			context += fmt.Sprintf("- Document: %s\n", a.FileName)
			if a.TextContent != "" {
				context += fmt.Sprintf("  Content excerpt: %s\n", a.TextContent[:min(500, len(a.TextContent))])
			}
		}
	}

	return context
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Prompt Enhancement ───────────────────────────────────────────────────────

// EnhancePrompt takes a rough user prompt and uses GLM to rewrite it into
// a more specific, detailed, and effective prompt for ad generation.
//
// Example:
//
//	Input:  "write something for my headphones"
//	Output: "Create a compelling 30-second ad script for premium wireless headphones
//	         targeting tech-savvy professionals aged 25-40. Use a formal tone that
//	         emphasizes sound quality, noise cancellation, and productivity benefits.
//	         Structure it with a strong hook about distractions, problem statement
//	         about open offices, solution highlighting the headphones, and a clear CTA."
func (cs *ChatService) EnhancePrompt(ctx context.Context, roughPrompt string, language string) (string, error) {
	metaPrompt := fmt.Sprintf(`You are a prompt engineering expert for an AI-powered audio advertisement platform.
 
A user has written this rough prompt:
"%s"
 
Rewrite it into a detailed, specific, and effective prompt that will produce a much better result.
The enhanced prompt should:
- Be specific about the product, target audience, and tone
- Include the Hook-Problem-Solution-CTA structure guidance
- Mention desired language (%s) and duration (30 seconds)
- Be concrete rather than vague
- Be 2-4 sentences long
 
Return ONLY the enhanced prompt text. No explanations, no "Here is the enhanced version:", just the prompt itself.`,
		roughPrompt, language,
	)

	// Pass context so cancellation propagates to GLM
	enhanced, err := cs.glmService.GenerateResponse(ctx, metaPrompt, "en")
	if err != nil {
		return "", fmt.Errorf("prompt enhancement failed: %w", err)
	}

	return enhanced, nil
}

// GetSessionWithLastMessage fetches a session and its most recent message
// for displaying in the sidebar preview.
func (cs *ChatService) GetSessionWithLastMessage(sessionID string) (*models.ChatSession, string, error) {
	var session models.ChatSession
	var lastMessage sql.NullString

	query := `
		SELECT s.id, s.user_id, COALESCE(s.campaign_id::text, ''), s.topic, s.status,
		       s.created_at, s.updated_at,
		       (SELECT content FROM chat_messages 
		        WHERE session_id = s.id 
		        ORDER BY created_at DESC LIMIT 1) as last_message
		FROM chat_sessions s
		WHERE s.id = $1
	`

	err := cs.db.QueryRow(query, sessionID).Scan(
		&session.ID, &session.UserID, &session.CampaignID,
		&session.Topic, &session.Status, &session.CreatedAt, &session.UpdatedAt,
		&lastMessage,
	)
	if err != nil {
		return nil, "", fmt.Errorf("session not found: %w", err)
	}

	preview := ""
	if lastMessage.Valid {
		preview = lastMessage.String
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
	}

	return &session, preview, nil
}

// buildChatPrompt builds context-aware prompt for GLM
func (cs *ChatService) buildChatPrompt(
	userMessage string,
	context *models.ChatContext,
	topic string,
) string {
	systemPrompt := ""

	switch topic {
	case "script_refinement":
		systemPrompt = `You are a professional copywriter helping refine marketing scripts.
    
Current script:
` + context.CurrentScript + `

User feedback: ` + userMessage + `

Provide specific, actionable suggestions to improve the script while maintaining the tone and target audience.`

	case "creative_ideas":
		systemPrompt = `You are a creative marketing strategist providing innovative campaign ideas.

Topic: ` + userMessage + `
Language: ` + context.Language + `
Tone: ` + context.Tone + `
Target Audience: ` + context.TargetAudience + `

Provide 2-3 creative campaign concepts with brief descriptions.`

	case "support":
		systemPrompt = `You are a helpful support assistant for Vocalize marketing platform.

User question: ` + userMessage + `

Provide clear, helpful guidance. If the issue requires human assistance, explain what information you'll need.`

	default:
		systemPrompt = `You are a helpful AI assistant for the Vocalize marketing platform.

User message: ` + userMessage

	}

	return systemPrompt
}

// ArchiveSession archives a chat session
func (cs *ChatService) ArchiveSession(sessionID string) error {
	query := `
    UPDATE chat_sessions
    SET status = 'archived', updated_at = CURRENT_TIMESTAMP
    WHERE id = $1
  `

	_, err := cs.db.Exec(query, sessionID)
	return err
}

// GenerateContextualResponse generates contextual AI response (for message editing)
func (cs *ChatService) GenerateContextualResponse(
	ctx context.Context,
	userContent string,
	sessionID string,
	context *models.ChatContext,
	topic string,
) (string, error) {
	// Fetch recent messages for context
	messages, err := cs.GetSessionHistory(sessionID, 5)
	if err != nil {
		return "", fmt.Errorf("failed to get context: %w", err)
	}

	// Extract previous messages as strings
	var previous []string
	for _, msg := range messages {
		previous = append(previous, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}

	// Dummy PromptContext from models.ChatContext
	promptCtx := &PromptContext{
		Language:       context.Language, // assume field exists
		CurrentScript:  context.CurrentScript,
		Tone:           context.Tone,
		TargetAudience: context.TargetAudience,
	}

	response, err := cs.glmService.GenerateContextualResponse(ctx, userContent, promptCtx, previous)
	if err != nil {
		return "", err
	}

	return response, nil
}
