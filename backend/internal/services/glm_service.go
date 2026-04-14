// backend/internal/services/glm_service.go (Enhanced)
package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GLMService struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
	promptMgr  *PromptContextManager
}

func NewGLMService(apiKey string, promptMgr *PromptContextManager) *GLMService {
	return &GLMService{
		apiKey:     apiKey,
		apiURL:     "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		httpClient: &http.Client{Timeout: 30 * time.Second},
		promptMgr:  promptMgr,
	}
}

// GenerateContextualResponse generates response with conversation context
func (gs *GLMService) GenerateResponse(prompt string, language string) (string, error) {
	return gs.callGLMAPI(prompt, language)
}

func (gs *GLMService) GenerateContextualResponse(

	userMessage string,
	context *PromptContext,
	previousMessages []string,
) (string, error) {
	// Build enriched prompt
	enrichedPrompt := gs.promptMgr.BuildEnrichedPrompt(
		userMessage,
		context,
		previousMessages,
	)

	// Call GLM API
	return gs.callGLMAPI(enrichedPrompt, context.Language)
}

// GenerateWithStreaming generates response with streaming (for long outputs)
func (gs *GLMService) GenerateWithStreaming(
	prompt string,
	language string,
	onChunk func(chunk string) error,
) error {
	payload := map[string]interface{}{
		"model": "glm-4-flash",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.7,
		"top_p":       0.95,
		"stream":      true,
		"max_tokens":  2000,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(
		"POST",
		gs.apiURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", gs.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := gs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Process streaming response
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		if line[:6] == "data: " {
			var streamEvent map[string]interface{}
			json.Unmarshal([]byte(line[6:]), &streamEvent)

			if choices, ok := streamEvent["choices"].([]interface{}); ok && len(choices) > 0 {
				choice := choices[0].(map[string]interface{})
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						onChunk(content)
					}
				}
			}
		}
	}

	return nil
}

func (gs *GLMService) callGLMAPI(prompt string, language string) (string, error) {
	payload := map[string]interface{}{
		"model": "glm-4-flash",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.7,
		"top_p":       0.95,
		"max_tokens":  2000,
	}

	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest(
		"POST",
		gs.apiURL,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", gs.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := gs.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	choices := result["choices"].([]interface{})
	if len(choices) == 0 {
		return "", fmt.Errorf("no response from GLM")
	}

	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	return message["content"].(string), nil
}
