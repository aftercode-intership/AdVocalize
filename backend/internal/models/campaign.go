// backend/internal/models/campaign.go
package models

import (
  "database/sql/driver"
  "encoding/json"
  "time"
)

type Campaign struct {
  ID             string    `json:"id"`
  UserID         string    `json:"user_id"`
  Name           string    `json:"name"`
  Brand          string    `json:"brand"`
  Objective      string    `json:"objective"` // AWARENESS, CONSIDERATION, CONVERSION, RETARGETING
  Description    string    `json:"description"`
  TargetMarkets  []string  `json:"target_markets"`
  Channels       []string  `json:"channels"`
  Budget         float64   `json:"budget"`
  Status         string    `json:"status"` // draft, in_progress, completed
  CreatedAt      time.Time `json:"created_at"`
  UpdatedAt      time.Time `json:"updated_at"`
}

type CampaignProduct struct {
  ID              string `json:"id"`
  CampaignID      string `json:"campaign_id"`
  ProductName     string `json:"product_name"`
  ProductDescription string `json:"product_description"`
  TargetAudience  string `json:"target_audience"`
  Tone            string `json:"tone"` // FORMAL, CASUAL, PODCAST
  Language        string `json:"language"` // en, fr, ar
  MarketingChannel string `json:"marketing_channel"`
  CreatedAt       time.Time `json:"created_at"`
  UpdatedAt       time.Time `json:"updated_at"`
}

// StringArray helper for PostgreSQL array type
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
  return json.Marshal(a)
}

func (a *StringArray) Scan(value interface{}) error {
  return json.Unmarshal(value.([]byte), &a)
}