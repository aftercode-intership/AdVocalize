// backend/internal/services/tts_service.go
package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TTSService is the Go-side client that calls the Python TTS microservice.
// It mirrors the same pattern as ScriptService — Go validates and proxies,
// Python does the actual work.
//
// Architecture:
//   Frontend → Go API (POST /api/generate/audio)
//              → TTSService.Generate()
//                → HTTP POST to Python service (http://tts:8000)
//                  → edge-tts → MP3 in MinIO
//              ← job_id returned immediately
//   Frontend polls GET /api/generate/status/:jobId
//              → TTSService.GetStatus()
//                → HTTP GET to Python (http://tts:8000/api/tts/status/:id)
//              ← status + audio_url when completed
type TTSService struct {
	db           *sql.DB
	ttsURL       string // e.g. "http://tts:8000" (Docker) or "http://localhost:8000" (local)
	creditService *CreditService
	httpClient   *http.Client
}

func NewTTSService(db *sql.DB, ttsURL string, creditService *CreditService) *TTSService {
	return &TTSService{
		db:            db,
		ttsURL:        ttsURL,
		creditService: creditService,
		httpClient: &http.Client{
			Timeout: 15 * time.Second, // just for the HTTP handshake; synthesis is async
		},
	}
}

// ── Request / Response types ──────────────────────────────────────────────────

// TTSGenerateRequest is the JSON body the frontend sends to the Go API.
type TTSGenerateRequest struct {
	ScriptText string  `json:"script_text"`
	Language   string  `json:"language"`   // en | fr | ar
	VoiceID    string  `json:"voice_id"`   // optional; service picks default if empty
	Speed      float64 `json:"speed"`      // 0.5–2.0, default 1.0
	AdID       string  `json:"ad_id"`      // optional: generated_ads.id
}

// TTSGenerateResponse is returned immediately to the frontend.
type TTSGenerateResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`  // always "queued" on creation
	Message string `json:"message"`
}

// TTSJobStatus is what we return when the frontend polls for status.
type TTSJobStatus struct {
	JobID           string   `json:"job_id"`
	Status          string   `json:"status"`           // queued|processing|completed|failed
	AudioURL        *string  `json:"audio_url"`        // presigned MinIO URL (when completed)
	DurationSeconds *float64 `json:"duration_seconds"` // actual audio length (when completed)
	FileSizeBytes   *int64   `json:"file_size_bytes"`
	VoiceID         *string  `json:"voice_id"`
	Language        *string  `json:"language"`
	Error           *string  `json:"error"`            // error message (when failed)
	CreatedAt       string   `json:"created_at"`
	CompletedAt     *string  `json:"completed_at"`
}

// TTSVoice represents a single available TTS voice.
type TTSVoice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Locale string `json:"locale"`
}

// ── Main methods ──────────────────────────────────────────────────────────────

// Generate enqueues a TTS synthesis job and returns the job ID.
// This call returns immediately — synthesis happens in the Python background.
func (ts *TTSService) Generate(ctx context.Context, userID string, req *TTSGenerateRequest) (*TTSGenerateResponse, error) {
	// Validate
	if len(req.ScriptText) < 5 {
		return nil, fmt.Errorf("script text must be at least 5 characters")
	}
	if req.Language == "" {
		req.Language = "en"
	}
	if req.Speed == 0 {
		req.Speed = 1.0
	}
	if req.Speed < 0.5 || req.Speed > 2.0 {
		return nil, fmt.Errorf("speed must be between 0.5 and 2.0")
	}

	// Check credits (1 credit per audio generation)
	balance, err := ts.creditService.GetBalance(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check credits: %w", err)
	}
	if balance != -1 && balance < 1 {
		return nil, fmt.Errorf("insufficient credits: you have %d credits remaining", balance)
	}

	// Call Python TTS service
	payload := map[string]interface{}{
		"script_text": req.ScriptText,
		"language":    req.Language,
		"voice_id":    req.VoiceID,
		"speed":       req.Speed,
		"ad_id":       req.AdID,
		"user_id":     userID,
	}

	bodyBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		ts.ttsURL+"/api/tts/generate",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := ts.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("TTS service unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("TTS service error (%d): %s", resp.StatusCode, errResp["detail"])
	}

	var result TTSGenerateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse TTS response: %w", err)
	}

	// Deduct 1 credit
	adIDPtr := (*string)(nil)
	if req.AdID != "" {
		adIDPtr = &req.AdID
	}
	go ts.creditService.DeductCredits(userID, 1, "TTS_GEN", adIDPtr)

	return &result, nil
}

// GetStatus polls the Python service for a job's current status.
func (ts *TTSService) GetStatus(ctx context.Context, jobID string) (*TTSJobStatus, error) {
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		ts.ttsURL+"/api/tts/status/"+jobID,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ts.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("TTS service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("job not found")
	}

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS service error (%d)", resp.StatusCode)
	}

	var status TTSJobStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &status, nil
}

// GetVoices returns available voices from the Python service.
func (ts *TTSService) GetVoices(ctx context.Context) (map[string][]TTSVoice, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.ttsURL+"/api/tts/voices", nil)
	if err != nil {
		return nil, err
	}

	resp, err := ts.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("TTS service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Voices map[string][]TTSVoice `json:"voices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Voices, nil
}