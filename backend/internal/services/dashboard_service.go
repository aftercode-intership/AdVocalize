// backend/internal/services/dashboard_service.go
package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DashboardService struct {
	db *sql.DB
}

func NewDashboardService(db *sql.DB) *DashboardService {
	return &DashboardService{db: db}
}

// DashboardStats is returned to the top-of-page stat cards.
type DashboardStats struct {
	TotalScripts    int     `json:"total_scripts"`
	TotalAudios     int     `json:"total_audios"`
	CreditsUsed     int     `json:"credits_used"`
	CreditsRemaining int    `json:"credits_remaining"`
	ScriptsThisWeek int     `json:"scripts_this_week"`
	Languages       map[string]int `json:"languages"` // {"en":5,"ar":3,"fr":2}
}

// RecentAd is one row in the library grid.
type RecentAd struct {
	ID                       string     `json:"id"`
	ProductName              string     `json:"product_name"`
	Language                 string     `json:"language"`
	Tone                     string     `json:"tone"`
	WordCount                int        `json:"word_count"`
	EstimatedDurationSeconds int        `json:"estimated_duration_seconds"`
	Status                   string     `json:"status"`
	ScriptPreview            string     `json:"script_preview"` // first 120 chars
	AudioURL                 *string    `json:"audio_url"`
	AudioDurationS           *float64   `json:"audio_duration_s"`
	AudioVoiceID             *string    `json:"audio_voice_id"`
	CreatedAt                time.Time  `json:"created_at"`
}

// AudioLinkRequest is the payload for saving audio metadata to a script record.
type AudioLinkRequest struct {
	AudioURL       string
	AudioJobID     string
	AudioVoiceID   string
	AudioDurationS float64
}

// GetStats returns headline numbers for the dashboard top cards.
func (ds *DashboardService) GetStats(ctx context.Context, userID string) (*DashboardStats, error) {
	stats := &DashboardStats{
		Languages: make(map[string]int),
	}

	// Total scripts generated
	err := ds.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM generated_ads WHERE user_id = $1 AND status = 'completed'`,
		userID,
	).Scan(&stats.TotalScripts)
	if err != nil {
		return nil, fmt.Errorf("failed to count scripts: %w", err)
	}

	// Total audios generated (scripts that have an audio_url)
	err = ds.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM generated_ads WHERE user_id = $1 AND audio_url IS NOT NULL`,
		userID,
	).Scan(&stats.TotalAudios)
	if err != nil {
		return nil, fmt.Errorf("failed to count audios: %w", err)
	}

	// Scripts generated this week
	err = ds.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM generated_ads
		 WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '7 days'`,
		userID,
	).Scan(&stats.ScriptsThisWeek)
	if err != nil {
		return nil, fmt.Errorf("failed to count weekly scripts: %w", err)
	}

	// Current credit balance
	err = ds.db.QueryRowContext(ctx,
		`SELECT credits_remaining FROM users WHERE id = $1`,
		userID,
	).Scan(&stats.CreditsRemaining)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get credits: %w", err)
	}

	// Credits used (sum of negative transactions)
	err = ds.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(ABS(amount)), 0)
		 FROM credit_transactions
		 WHERE user_id = $1 AND amount < 0`,
		userID,
	).Scan(&stats.CreditsUsed)
	if err != nil {
		stats.CreditsUsed = 0 // non-fatal
	}

	// Language breakdown
	rows, err := ds.db.QueryContext(ctx,
		`SELECT language, COUNT(*) as cnt
		 FROM generated_ads
		 WHERE user_id = $1 AND status = 'completed'
		 GROUP BY language`,
		userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lang string
			var cnt int
			if err := rows.Scan(&lang, &cnt); err == nil {
				stats.Languages[lang] = cnt
			}
		}
	}

	return stats, nil
}

// GetRecentAds returns the user's most recent generated ads.
func (ds *DashboardService) GetRecentAds(ctx context.Context, userID string, limit int) ([]RecentAd, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	rows, err := ds.db.QueryContext(ctx, `
		SELECT
			id,
			product_name,
			language,
			tone,
			COALESCE(word_count, 0),
			COALESCE(estimated_duration_seconds, 0),
			status,
			LEFT(script_text, 120) as script_preview,
			audio_url,
			audio_duration_s,
			audio_voice_id,
			created_at
		FROM generated_ads
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query ads: %w", err)
	}
	defer rows.Close()

	var ads []RecentAd
	for rows.Next() {
		var ad RecentAd
		if err := rows.Scan(
			&ad.ID,
			&ad.ProductName,
			&ad.Language,
			&ad.Tone,
			&ad.WordCount,
			&ad.EstimatedDurationSeconds,
			&ad.Status,
			&ad.ScriptPreview,
			&ad.AudioURL,
			&ad.AudioDurationS,
			&ad.AudioVoiceID,
			&ad.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ad: %w", err)
		}
		ads = append(ads, ad)
	}

	return ads, nil
}

// LinkAudio persists the completed TTS audio URL to a generated_ads row.
func (ds *DashboardService) LinkAudio(ctx context.Context, adID, userID string, req AudioLinkRequest) error {
	result, err := ds.db.ExecContext(ctx, `
		UPDATE generated_ads
		SET
			audio_url          = $1,
			audio_job_id       = $2,
			audio_voice_id     = $3,
			audio_duration_s   = $4,
			audio_generated_at = NOW(),
			updated_at         = NOW()
		WHERE id = $5 AND user_id = $6
	`,
		req.AudioURL,
		req.AudioJobID,
		req.AudioVoiceID,
		req.AudioDurationS,
		adID,
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to link audio: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("ad not found or not owned by user")
	}

	return nil
}