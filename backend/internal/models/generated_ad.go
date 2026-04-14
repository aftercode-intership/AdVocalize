package models

import "time"

type GeneratedAd struct {
	ID                       string    `json:"id"`
	CampaignID               string    `json:"campaign_id"`
	UserID                   string    `json:"user_id"`
	ProductName              string    `json:"product_name"`
	ProductDescription       string    `json:"product_description"`
	TargetAudience           string    `json:"target_audience"`
	Tone                     string    `json:"tone"`
	Language                 string    `json:"language"`
	ScriptText               string    `json:"script_text"`
	WordCount                int       `json:"word_count"`
	EstimatedDurationSeconds int       `json:"estimated_duration_seconds"`
	Status                   string    `json:"status"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}
