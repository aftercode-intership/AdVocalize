// backend/internal/services/prompt_context_manager.go
package services

import (
	"database/sql"
	"fmt"
	"strings"
)

type PromptContextManager struct {
	db *sql.DB
}

func NewPromptContextManager(db *sql.DB) *PromptContextManager {
	return &PromptContextManager{db: db}
}

// BuildEnrichedPrompt builds context-aware prompt with conversation history
func (pcm *PromptContextManager) BuildEnrichedPrompt(
	userMessage string,
	context *PromptContext,
	previousMessages []string,
) string {
	var prompt strings.Builder

	// System role based on topic
	systemRole := pcm.getSystemRole(context.Topic)
	prompt.WriteString(systemRole)
	prompt.WriteString("\n\n")

	// Conversation context
	prompt.WriteString("=== CONVERSATION HISTORY ===\n")
	for _, msg := range previousMessages {
		prompt.WriteString(msg)
		prompt.WriteString("\n")
	}
	prompt.WriteString("\n")

	// Current script context (if available)
	if context.CurrentScript != "" {
		prompt.WriteString("=== CURRENT SCRIPT ===\n")
		prompt.WriteString(context.CurrentScript)
		prompt.WriteString("\n\n")
	}

	// Campaign context
	prompt.WriteString("=== CAMPAIGN CONTEXT ===\n")
	prompt.WriteString(fmt.Sprintf("Language: %s\n", context.Language))
	prompt.WriteString(fmt.Sprintf("Tone: %s\n", context.Tone))
	prompt.WriteString(fmt.Sprintf("Target Audience: %s\n", context.TargetAudience))
	if context.ProductName != "" {
		prompt.WriteString(fmt.Sprintf("Product: %s\n", context.ProductName))
	}
	if context.BrandGuidelines != "" {
		prompt.WriteString(fmt.Sprintf("Brand Guidelines: %s\n", context.BrandGuidelines))
	}
	prompt.WriteString("\n")

	// User's current request
	prompt.WriteString("=== USER REQUEST ===\n")
	prompt.WriteString(userMessage)
	prompt.WriteString("\n\n")

	// Instructions
	prompt.WriteString("=== INSTRUCTIONS ===\n")
	prompt.WriteString(pcm.getInstructions(context.Topic))

	return prompt.String()
}

func (pcm *PromptContextManager) getSystemRole(topic string) string {
	switch topic {
	case "script_refinement":
		return `You are an expert copywriter and marketing strategist specializing in creating compelling advertising scripts. 
Your role is to provide detailed feedback and suggestions for improving marketing copy. 
Be specific, actionable, and always explain the "why" behind your recommendations.`

	case "creative_ideas":
		return `You are a creative marketing strategist with expertise in developing innovative advertising campaigns.
Your role is to generate original, compelling ideas that resonate with target audiences.
Consider market trends, consumer psychology, and cultural relevance in your suggestions.`

	case "support":
		return `You are a knowledgeable support specialist for the Vocalize marketing platform.
Your role is to help users troubleshoot issues, answer questions, and provide guidance on using the platform effectively.
Be clear, empathetic, and always provide actionable solutions.`

	default:
		return "You are a helpful AI assistant."
	}
}

func (pcm *PromptContextManager) getInstructions(topic string) string {
	switch topic {
	case "script_refinement":
		return `Provide 3-5 specific suggestions for improving the script. For each suggestion:
1. Identify the issue (e.g., weak hook, unclear CTA)
2. Explain why it's a problem
3. Provide a concrete example of improvement
4. Estimate impact on effectiveness (e.g., "likely to increase engagement by 15-20%")

Format your response as a numbered list with clear sections.`

	case "creative_ideas":
		return `Generate 2-3 creative campaign concepts. For each concept:
1. Give it a memorable name
2. Provide a 2-3 sentence description
3. Explain the core message and emotional appeal
4. Suggest visual/audio elements
5. Identify the key target segment

Be innovative but realistic for a marketing agency to execute.`

	case "support":
		return `If you can resolve the issue immediately, provide a step-by-step solution.
If you need clarification, ask specific questions.
If the issue requires human assistance, explain what information you'll need to escalate.

Always maintain a professional, helpful tone.`

	default:
		return ""
	}
}

// GetRelevantPreviousMessages retrieves relevant context from conversation history
func (pcm *PromptContextManager) GetRelevantPreviousMessages(
	sessionID string,
	maxMessages int,
) ([]string, error) {
	query := `
    SELECT content, role
    FROM chat_messages
    WHERE session_id = $1
    ORDER BY created_at DESC
    LIMIT $2
  `

	rows, err := pcm.db.Query(query, sessionID, maxMessages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var content, role string
		if err := rows.Scan(&content, &role); err != nil {
			return nil, err
		}

		prefix := "User:"
		if role == "ASSISTANT" {
			prefix = "Assistant:"
		}

		messages = append([]string{fmt.Sprintf("%s %s", prefix, content)}, messages...)
	}

	return messages, nil
}

type PromptContext struct {
	Topic           string
	CurrentScript   string
	Language        string
	Tone            string
	TargetAudience  string
	ProductName     string
	BrandGuidelines string
}
