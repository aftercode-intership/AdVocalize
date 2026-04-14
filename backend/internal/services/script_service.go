// backend/internal/services/script_service.go
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

// ScriptService is the Go-side client that calls the Python script-gen
// microservice. It also saves results to the database and deducts credits.
//
// Architecture reminder:
//   Frontend → Go API (/api/generate/script)
//              → ScriptService.Generate()
//                → HTTP POST to Python service (http://script-gen:8001)
//                  → GLM Flash 4.7 API
//              ← script text returned
//              → saved to generated_ads table
//              → 1 credit deducted
type ScriptService struct {
	db            *sql.DB
	scriptGenURL  string        // e.g. "http://script-gen:8001"
	creditService *CreditService
	httpClient    *http.Client
}

func NewScriptService(db *sql.DB, scriptGenURL string, creditService *CreditService) *ScriptService {
	return &ScriptService{
		db:           db,
		scriptGenURL: scriptGenURL,
		creditService: creditService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ── Request / Response types (mirror the Python Pydantic models) ────────────

// ScriptGenerateRequest is the JSON body the frontend sends to the Go API.
type ScriptGenerateRequest struct {
	ProductName        string `json:"product_name"`
	ProductDescription string `json:"product_description"`
	TargetAudience     string `json:"target_audience"`
	Tone               string `json:"tone"`      // FORMAL | CASUAL | PODCAST
	Language           string `json:"language"`  // en | fr | ar
	CampaignID         string `json:"campaign_id,omitempty"`
	BrandGuidelines    string `json:"brand_guidelines,omitempty"`
}

// ScriptGenerateResponse is what we return to the frontend.
type ScriptGenerateResponse struct {
	ID                       string    `json:"id"`
	ScriptText               string    `json:"script_text"`
	Language                 string    `json:"language"`
	Tone                     string    `json:"tone"`
	WordCount                int       `json:"word_count"`
	EstimatedDurationSeconds int       `json:"estimated_duration_seconds"`
	Version                  int       `json:"version"`
	Status                   string    `json:"status"`
	CreatedAt                time.Time `json:"created_at"`
}

// pythonScriptResponse mirrors the Python ScriptResponse Pydantic model.
type pythonScriptResponse struct {
	ScriptText               string `json:"script_text"`
	Language                 string `json:"language"`
	Tone                     string `json:"tone"`
	WordCount                int    `json:"word_count"`
	EstimatedDurationSeconds int    `json:"estimated_duration_seconds"`
	Version                  int    `json:"version"`
}

// ── Main method ─────────────────────────────────────────────────────────────

// Generate calls the Python script-gen service, saves the result to DB,
// and deducts 1 credit from the user.
func (ss *ScriptService) Generate(
	ctx context.Context,
	userID string,
	req *ScriptGenerateRequest,
) (*ScriptGenerateResponse, error) {

	// 1. Validate inputs before hitting the Python service
	if err := ss.validate(req); err != nil {
		return nil, err
	}

	// 2. Check user has credits before doing any expensive work
	balance, err := ss.creditService.GetBalance(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check credits: %w", err)
	}
	// -1 means unlimited (Pro tier)
	if balance != -1 && balance < 1 {
		return nil, fmt.Errorf("insufficient credits: you have %d credits remaining", balance)
	}

	// 3. Call Python script-gen service
	pyResponse, err := ss.callPythonService(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("script generation failed: %w", err)
	}

	// 4. Save to database
	adID, err := ss.saveToDatabase(userID, req, pyResponse)
	if err != nil {
		// Log but don't fail — the script was generated successfully,
		// we just couldn't save it. Return the result anyway.
		fmt.Printf("Warning: failed to save generated script to DB: %v\n", err)
	}

	// 5. Deduct 1 credit atomically
	campaignIDPtr := (*string)(nil)
	if req.CampaignID != "" {
		campaignIDPtr = &req.CampaignID
	}
	if err := ss.creditService.DeductCredits(userID, 1, "SCRIPT_GEN", campaignIDPtr); err != nil {
		// Log but don't fail — the script is already generated
		fmt.Printf("Warning: failed to deduct credit for user %s: %v\n", userID, err)
	}

	return &ScriptGenerateResponse{
		ID:                       adID,
		ScriptText:               pyResponse.ScriptText,
		Language:                 pyResponse.Language,
		Tone:                     pyResponse.Tone,
		WordCount:                pyResponse.WordCount,
		EstimatedDurationSeconds: pyResponse.EstimatedDurationSeconds,
		Version:                  pyResponse.Version,
		Status:                   "completed",
		CreatedAt:                time.Now(),
	}, nil
}

// ── Private helpers ──────────────────────────────────────────────────────────

func (ss *ScriptService) validate(req *ScriptGenerateRequest) error {
	if len(req.ProductName) < 2 {
		return fmt.Errorf("product name must be at least 2 characters")
	}
	if len(req.ProductDescription) < 10 {
		return fmt.Errorf("product description must be at least 10 characters")
	}
	if len(req.TargetAudience) < 5 {
		return fmt.Errorf("target audience must be at least 5 characters")
	}

	validTones := map[string]bool{"FORMAL": true, "CASUAL": true, "PODCAST": true}
	if !validTones[req.Tone] {
		return fmt.Errorf("tone must be FORMAL, CASUAL, or PODCAST")
	}

	validLanguages := map[string]bool{"en": true, "fr": true, "ar": true}
	if !validLanguages[req.Language] {
		return fmt.Errorf("language must be en, fr, or ar")
	}

	return nil
}

func (ss *ScriptService) callPythonService(
	ctx context.Context,
	req *ScriptGenerateRequest,
) (*pythonScriptResponse, error) {

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		ss.scriptGenURL+"/api/generate/script",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := ss.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("script-gen service unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Surface Python service errors clearly
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.Unmarshal(respBody, &errResp)
		detail := errResp["detail"]
		if detail == "" {
			detail = string(respBody)
		}
		return nil, fmt.Errorf("script-gen error (%d): %s", resp.StatusCode, detail)
	}

	var pyResp pythonScriptResponse
	if err := json.Unmarshal(respBody, &pyResp); err != nil {
		return nil, fmt.Errorf("failed to parse script-gen response: %w", err)
	}

	return &pyResp, nil
}

func (ss *ScriptService) saveToDatabase(
	userID string,
	req *ScriptGenerateRequest,
	resp *pythonScriptResponse,
) (string, error) {

	var id string
	query := `
		INSERT INTO generated_ads (
			user_id, campaign_id, product_name, product_description,
			target_audience, tone, language, script_text,
			word_count, estimated_duration_seconds, status,
			created_at, updated_at
		) VALUES (
			$1, NULLIF($2, ''), $3, $4,
			$5, $6, $7, $8,
			$9, $10, 'completed',
			NOW(), NOW()
		)
		RETURNING id
	`

	err := ss.db.QueryRow(
		query,
		userID, req.CampaignID, req.ProductName, req.ProductDescription,
		req.TargetAudience, req.Tone, req.Language, resp.ScriptText,
		resp.WordCount, resp.EstimatedDurationSeconds,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("DB insert failed: %w", err)
	}

	return id, nil
}

// GetGeneratedScript retrieves a previously generated script by ID.
func (ss *ScriptService) GetGeneratedScript(ctx context.Context, adID, userID string) (*ScriptGenerateResponse, error) {
	var resp ScriptGenerateResponse

	query := `
		SELECT id, script_text, language, tone, word_count,
		       estimated_duration_seconds, status, created_at
		FROM generated_ads
		WHERE id = $1 AND user_id = $2
	`

	err := ss.db.QueryRowContext(ctx, query, adID, userID).Scan(
		&resp.ID, &resp.ScriptText, &resp.Language, &resp.Tone,
		&resp.WordCount, &resp.EstimatedDurationSeconds,
		&resp.Status, &resp.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("script not found")
	}
	if err != nil {
		return nil, fmt.Errorf("DB query failed: %w", err)
	}

	return &resp, nil
}