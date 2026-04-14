package services

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"vocalize/internal/models"
)

type ProfileService struct {
	db *sql.DB
}

func NewProfileService(db *sql.DB) *ProfileService {
	return &ProfileService{db: db}
}

func (ps *ProfileService) GetProfile(userID string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, email, email_verified, name, company, bio, language, avatar_url, subscription_tier, credits_remaining, account_status, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	err := ps.db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerified,
		&user.Name,
		&user.Company,
		&user.Bio,
		&user.Language,
		&user.Avatar,
		&user.SubscriptionTier,
		&user.CreditsRemaining,
		&user.AccountStatus,
		&user.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	return user, nil
}

func (ps *ProfileService) UpdateProfile(userID, name, company, bio, language string) (*models.User, error) {
	query := `
		UPDATE users 
		SET name = $1, company = $2, bio = $3, language = $4 
		WHERE id = $5
	`
	_, err := ps.db.Exec(query, name, company, bio, language, userID)
	if err != nil {
		return nil, err
	}
	return ps.GetProfile(userID)
}

func (ps *ProfileService) UpdateAvatarURL(userID, url string) error {
	query := `UPDATE users SET avatar_url = $1 WHERE id = $2`
	_, err := ps.db.Exec(query, url, userID)
	return err
}

func (ps *ProfileService) ExportUserData(userID string) ([]byte, error) {
	user, err := ps.GetProfile(userID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(user)
}
