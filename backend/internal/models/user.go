package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID                string           `json:"id" db:"id"`
	Name              string           `json:"name" db:"name"`
	Email             string           `json:"email" db:"email"`
	PasswordHash      string           `json:"-" db:"password_hash"`
	EmailVerified     bool             `json:"email_verified" db:"email_verified"`
	EmailVerifiedAt   *time.Time       `json:"email_verified_at" db:"email_verified_at"`
	Avatar            *string          `json:"avatar" db:"avatar_url"`
	Company           *string          `json:"company" db:"company"`
	Bio               *string          `json:"bio" db:"bio"`
	Language          string           `json:"language" db:"language"`                   // en, fr, ar
	SubscriptionTier  string           `json:"subscription_tier" db:"subscription_tier"` // FREE, PRO
	CreditsRemaining  int              `json:"credits_remaining" db:"credits_remaining"`
	AccountStatus     string           `json:"account_status" db:"account_status"` // active, suspended, deleted
	GoogleID          *string          `json:"-" db:"google_id"`
	GoogleProfileData *json.RawMessage `json:"-" db:"google_profile_data"`
	CreatedAt         time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at" db:"updated_at"`
	DeletedAt         *time.Time       `json:"deleted_at" db:"deleted_at"`
}

type Session struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"user_id" db:"user_id"`
	TokenHash      string    `json:"-" db:"token_hash"`
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at" db:"last_activity_at"`
	IPAddress      *string   `json:"ip_address" db:"ip_address"`
	UserAgent      *string   `json:"user_agent" db:"user_agent"`
}

type EmailVerificationToken struct {
	ID        string     `json:"id" db:"id"`
	UserID    string     `json:"user_id" db:"user_id"`
	Token     string     `json:"-" db:"token"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UsedAt    *time.Time `json:"used_at" db:"used_at"`
}

// Response DTOs
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=255"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	EmailVerified    bool   `json:"email_verified"`
	SubscriptionTier string `json:"subscription_tier"`
	CreditsRemaining int    `json:"credits_remaining"`
	Token            string `json:"token,omitempty"`
	ExpiresIn        int    `json:"expires_in,omitempty"`
}

type GoogleOAuthResponse struct {
	Sub           string `json:"sub"` // Google user ID
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture"`
}
