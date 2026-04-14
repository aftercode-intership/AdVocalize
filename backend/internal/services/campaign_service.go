// backend/internal/services/campaign_service.go
package services

import (
	"database/sql"
	"fmt"

	"vocalize/internal/models"

	"github.com/google/uuid"
)

type CampaignService struct {
	db *sql.DB
}

func NewCampaignService(db *sql.DB) *CampaignService {
	return &CampaignService{db: db}
}

func (cs *CampaignService) CreateCampaign(userID string, campaign *models.Campaign) (*models.Campaign, error) {
	query := `
    INSERT INTO campaigns (id, user_id, name, brand, objective, description, target_markets, channels, budget, status, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id, user_id, name, brand, objective, description, target_markets, channels, budget, status, created_at, updated_at
  `

	campaign.ID = generateUUID()
	campaign.UserID = userID
	campaign.Status = "draft"

	err := cs.db.QueryRow(
		query,
		campaign.ID, userID, campaign.Name, campaign.Brand, campaign.Objective,
		campaign.Description, campaign.TargetMarkets, campaign.Channels, campaign.Budget, "draft",
	).Scan(
		&campaign.ID, &campaign.UserID, &campaign.Name, &campaign.Brand, &campaign.Objective,
		&campaign.Description, &campaign.TargetMarkets, &campaign.Channels, &campaign.Budget, &campaign.Status,
		&campaign.CreatedAt, &campaign.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}

	return campaign, nil
}

func (cs *CampaignService) SaveProductForm(product *models.CampaignProduct) error {
	query := `
    INSERT INTO campaign_products (id, campaign_id, product_name, product_description, target_audience, tone, language, marketing_channel, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
  `

	_, err := cs.db.Exec(
		query,
		product.ID, product.CampaignID, product.ProductName, product.ProductDescription,
		product.TargetAudience, product.Tone, product.Language, product.MarketingChannel,
	)

	if err != nil {
		return fmt.Errorf("failed to save product form: %w", err)
	}

	return nil
}

func (cs *CampaignService) GetCampaign(campaignID string) (*models.Campaign, error) {
	campaign := &models.Campaign{}
	query := `
    SELECT id, user_id, name, brand, objective, description, target_markets, channels, budget, status, created_at, updated_at
    FROM campaigns
    WHERE id = $1
  `

	err := cs.db.QueryRow(query, campaignID).Scan(
		&campaign.ID, &campaign.UserID, &campaign.Name, &campaign.Brand, &campaign.Objective,
		&campaign.Description, &campaign.TargetMarkets, &campaign.Channels, &campaign.Budget, &campaign.Status,
		&campaign.CreatedAt, &campaign.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("campaign not found")
		}
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	return campaign, nil
}

func (cs *CampaignService) UserOwnsCampaign(userID, campaignID string) (bool, error) {
	var id string
	query := `SELECT id FROM campaigns WHERE id = $1 AND user_id = $2`

	err := cs.db.QueryRow(query, campaignID, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func generateUUID() string {
	return uuid.New().String()
}
