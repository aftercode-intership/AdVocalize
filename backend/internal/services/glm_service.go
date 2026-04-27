// backend/internal/services/glm_service.go (NVIDIA Fixed - Syntax Clean)
package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type GLMService struct {
	nvidiaKey     string
	openrouterKey string
	apiKey        string // current active key
	apiURL        string
	model         string
	streamModel   string
	httpClient    *http.Client
	promptMgr     *PromptContextManager
	environment   string
}

func NewGLMService(nvidiaKey, openrouterKey, provider, environment string, promptMgr *PromptContextManager) *GLMService {
	apiURL := "https://openrouter.ai/api/v1/chat/completions"
	model := "z-ai/glm-5.1"
	activeKey := openrouterKey

	if provider == "nvidia" {
		apiURL = "https://integrate.api.nvidia.com/v1/chat/completions"
		model = os.Getenv("GLM_MODEL_NONSTREAM")
		if model == "" {
			model = "z-ai/glm-5.1"
		}
		activeKey = nvidiaKey
	}

	streamModel := os.Getenv("GLM_MODEL_STREAM")
	if streamModel == "" {
		streamModel = "glm-4-flash"
	}
	return &GLMService{
		nvidiaKey:     nvidiaKey,
		openrouterKey: openrouterKey,
		apiKey:        activeKey,
		apiURL:        apiURL,
		model:         model,
		streamModel:   streamModel,
		// Reduced timeout: 25s per attempt allows 3 attempts + backoff within 90s WS timeout
		httpClient:  &http.Client{Timeout: 25 * time.Second},
		promptMgr:   promptMgr,
		environment: environment,
	}
}

// GenerateResponse generates a response with optional context cancellation.
func (gs *GLMService) GenerateResponse(ctx context.Context, prompt string, language string) (string, error) {
	return gs.callGLMAPI(ctx, prompt, language)
}

func (gs *GLMService) GenerateContextualResponse(
	ctx context.Context,
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
	return gs.callGLMAPI(ctx, enrichedPrompt, context.Language)
}

// GenerateWithStreaming generates response with streaming (for long outputs)
func (gs *GLMService) GenerateWithStreaming(
	ctx context.Context,
	prompt string,
	language string,
	onChunk func(chunk string) error,
) error {
	// Development mock mode when no API keys are configured
	if gs.apiKey == "" && gs.environment == "development" {
		log.Println("[GLM] Mock mode: streaming development placeholder response")
		mockResponse := fmt.Sprintf("[DEV MOCK STREAM] Received prompt in %s. This is a simulated response for local development without an API key.", language)
		// Stream it in chunks to simulate real behavior
		chunks := strings.Split(mockResponse, " ")
		for _, chunk := range chunks {
			if err := onChunk(chunk + " "); err != nil {
				return err
			}
			time.Sleep(50 * time.Millisecond)
		}
		return nil
	}

	payload := map[string]interface{}{
		"model": gs.streamModel,
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
		"extra_body": map[string]interface{}{
			"chat_template_kwargs": map[string]bool{
				"enable_thinking": true,
				"clear_thinking":  false,
			},
		},
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
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

func (gs *GLMService) callGLMAPI(ctx context.Context, prompt string, language string) (string, error) {
	// Development mock mode when no API keys are configured
	if gs.apiKey == "" && gs.environment == "development" {
		log.Println("[GLM] Mock mode: returning development placeholder response")
		return fmt.Sprintf("[DEV MOCK] This is a simulated response for local development. Language: %s. Your prompt was: %s", language, prompt), nil
	}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Respect caller cancellation before each attempt
		if ctx.Err() != nil {
			return "", fmt.Errorf("request cancelled: %w", ctx.Err())
		}

		payload := map[string]interface{}{
			"model": gs.model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"temperature": 0.7,
			"top_p":       0.95,
			"max_tokens":  2000,
			"extra_body": map[string]interface{}{
				"chat_template_kwargs": map[string]bool{
					"enable_thinking": true,
					"clear_thinking":  false,
				},
			},
		}

		jsonBody, _ := json.Marshal(payload)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, gs.apiURL, bytes.NewReader(jsonBody))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", gs.apiKey))
		req.Header.Set("Content-Type", "application/json")

		resp, err := gs.httpClient.Do(req)
		if err != nil {
			log.Printf("[GLM] Attempt %d/%d failed (network): %v", attempt+1, maxRetries, err)

			// ── CRITICAL FIX: On ANY error with NVIDIA, fallback to OpenRouter if key available ──
			if strings.Contains(gs.apiURL, "nvidia.com") && gs.openrouterKey != "" {
				log.Println("[GLM] NVIDIA error detected, falling back to OpenRouter")
				gs.apiURL = "https://openrouter.ai/api/v1/chat/completions"
				gs.model = "z-ai/glm-5.1"
				gs.apiKey = gs.openrouterKey
				// Do not count this as a retry against OpenRouter
				attempt--
				continue
			}

			if attempt < maxRetries-1 {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				time.Sleep(backoff)
				continue
			}
			return "", fmt.Errorf("API call failed after %d attempts: %w", maxRetries, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("API error %d: %s", resp.StatusCode, string(body))
			log.Printf("[GLM] %s (URL: %s)", errMsg, gs.apiURL)

			// ── CRITICAL FIX: On ANY non-OK status with NVIDIA, fallback to OpenRouter if key available ──
			if strings.Contains(gs.apiURL, "nvidia.com") && gs.openrouterKey != "" {
				log.Println("[GLM] NVIDIA non-OK status, falling back to OpenRouter")
				gs.apiURL = "https://openrouter.ai/api/v1/chat/completions"
				gs.model = "z-ai/glm-5.1"
				gs.apiKey = gs.openrouterKey
				attempt--
				continue
			}

			return "", fmt.Errorf(errMsg)
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
	return "", fmt.Errorf("max retries exceeded")
}
