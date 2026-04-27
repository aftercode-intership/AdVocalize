// backend/internal/handlers/campaign_handler.go
package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"vocalize/internal/models"
	"vocalize/internal/services"
)

type CreateCampaignRequest struct {
	Name          string   `json:"name" validate:"required,min=3,max=255"`
	Brand         string   `json:"brand" validate:"required,min=2,max=255"`
	Objective     string   `json:"objective" validate:"required,oneof=AWARENESS CONSIDERATION CONVERSION RETARGETING"`
	Description   string   `json:"description" validate:"max=1000"`
	TargetMarkets []string `json:"target_markets" validate:"required"`
	Channels      []string `json:"channels" validate:"required"`
	Budget        float64  `json:"budget" validate:"required,gt=0"`
}

type ProductFormRequest struct {
	CampaignID        string `json:"campaign_id" validate:"required"`
	ProductName       string `json:"product_name" validate:"required,min=2,max=255"`
	ProductDescription string `json:"product_description" validate:"required,min=10,max=1000"`
	TargetAudience    string `json:"target_audience" validate:"required,min=5,max=255"`
	Tone              string `json:"tone" validate:"required,oneof=FORMAL CASUAL PODCAST"`
	Language          string `json:"language" validate:"required,oneof=en fr ar"`
	MarketingChannel  string `json:"marketing_channel" validate:"required,oneof=YouTube Instagram TikTok Spotify Programmatic"`
}

type CampaignResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Brand         string   `json:"brand"`
	Objective     string   `json:"objective"`
	TargetMarkets []string `json:"target_markets"`
	Channels      []string `json:"channels"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"created_at"`
}

type CampaignHandler struct {
	campaignService *services.CampaignService
}

func NewCampaignHandler(campaignService *services.CampaignService) *CampaignHandler {
	return &CampaignHandler{
		campaignService: campaignService,
	}
}

// CreateCampaign handles POST /api/campaigns
func (h *CampaignHandler) CreateCampaign(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	var req CreateCampaignRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if len(req.Name) < 3 || len(req.Name) > 255 {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Name must be between 3 and 255 characters",
		})
	}

	if len(req.Brand) < 2 || len(req.Brand) > 255 {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Brand must be between 2 and 255 characters",
		})
	}

	if req.Budget <= 0 {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Budget must be greater than 0",
		})
	}

	// Validate target markets (valid country codes)
	for _, market := range req.TargetMarkets {
		if !isValidCountryCode(market) {
			return c.Status(http.StatusBadRequest).JSON(map[string]string{
				"error": "Invalid country code: " + market,
			})
		}
	}

	campaign, err := h.campaignService.CreateCampaign(userID, &models.Campaign{
		Name:          req.Name,
		Brand:         req.Brand,
		Objective:     req.Objective,
		Description:   req.Description,
		TargetMarkets: req.TargetMarkets,
		Channels:      req.Channels,
		Budget:        req.Budget,
	})

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to create campaign",
		})
	}

	return c.Status(http.StatusCreated).JSON(CampaignResponse{
		ID:            campaign.ID,
		Name:          campaign.Name,
		Brand:         campaign.Brand,
		Objective:     campaign.Objective,
		TargetMarkets: campaign.TargetMarkets,
		Channels:      campaign.Channels,
		Status:        campaign.Status,
		CreatedAt:     campaign.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// SaveProductForm handles POST /api/campaigns/:id/product-form
func (h *CampaignHandler) SaveProductForm(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	campaignID := c.Params("id")

	var req ProductFormRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if len(req.ProductName) < 2 || len(req.ProductName) > 255 {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{
			"error": "Product name must be between 2 and 255 characters",
		})
	}

	// Verify user owns this campaign
	owns, err := h.campaignService.UserOwnsCampaign(userID, campaignID)
	if err != nil || !owns {
		return c.Status(http.StatusForbidden).JSON(map[string]string{
			"error": "Cannot access this campaign",
		})
	}

	product := &models.CampaignProduct{
		ID:                 uuid.New().String(),
		CampaignID:         campaignID,
		ProductName:        req.ProductName,
		ProductDescription: req.ProductDescription,
		TargetAudience:     req.TargetAudience,
		Tone:               req.Tone,
		Language:           req.Language,
		MarketingChannel:   req.MarketingChannel,
	}

	if err := h.campaignService.SaveProductForm(product); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to save product form",
		})
	}

	return c.Status(http.StatusCreated).JSON(product)
}

// GetCampaign handles GET /api/campaigns/:id
func (h *CampaignHandler) GetCampaign(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	campaignID := c.Params("id")

	campaign, err := h.campaignService.GetCampaign(campaignID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(map[string]string{
			"error": "Campaign not found",
		})
	}

	// Verify user owns campaign
	if campaign.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(map[string]string{
			"error": "Cannot access this campaign",
		})
	}

	return c.Status(http.StatusOK).JSON(campaign)
}

// Helper functions
func isValidCountryCode(code string) bool {
	validCodes := map[string]bool{
		"US": true, "GB": true, "FR": true, "DE": true, "ES": true,
		"IT": true, "CA": true, "AU": true, "JP": true, "BR": true,
		"IN": true, "ZA": true, "MX": true, "KR": true, "SG": true,
		"AE": true, "SA": true, "EG": true, "AR": true, "NZ": true,
		"BE": true, "NL": true, "SE": true, "NO": true, "DK": true,
		"CH": true, "AT": true, "TR": true, "IL": true, "TH": true,
		"MY": true, "ID": true, "VN": true, "PH": true, "PK": true,
	}
	return validCodes[code]
}
