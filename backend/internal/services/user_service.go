// backend/internal/services/user_service.go
package services

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"vocalize/internal/models"
)

type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

func (us *UserService) IsAdmin(userID string) (bool, error) {
	// Check against admin list from environment
	adminIDs := os.Getenv("ADMIN_USER_IDS")
	if adminIDs == "" {
		return false, nil
	}

	admins := strings.Split(adminIDs, ",")
	for _, admin := range admins {
		if strings.TrimSpace(admin) == userID {
			return true, nil
		}
	}

	return false, nil
}

func (us *UserService) GetUserByID(userID string) (*models.User, error) {
	user := &models.User{}
	query := `
    SELECT id, email, email_verified, name, language, subscription_tier, credits_remaining, account_status, created_at
    FROM users
    WHERE id = $1 AND deleted_at IS NULL
  `

	err := us.db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerified,
		&user.Name,
		&user.Language,
		&user.SubscriptionTier,
		&user.CreditsRemaining,
		&user.AccountStatus,
		&user.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}
