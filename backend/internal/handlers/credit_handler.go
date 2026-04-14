// backend/internal/handlers/credit_handler.go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"vocalize/internal/services"
)

type CreditHandler struct {
	creditService *services.CreditService
}

func NewCreditHandler(creditService *services.CreditService) *CreditHandler {
	return &CreditHandler{creditService: creditService}
}

type BalanceResponse struct {
	CreditsRemaining int `json:"credits_remaining"`
	MonthlyLimit     int `json:"monthly_limit"`
	UsageThisMonth   int `json:"usage_this_month"`
}

type TransactionResponse struct {
	ID           string `json:"id"`
	Amount       int    `json:"amount"`
	Reason       string `json:"reason"`
	CampaignID   string `json:"campaign_id,omitempty"`
	BalanceAfter int    `json:"balance_after"`
	CreatedAt    string `json:"created_at"`
}

// GetBalance handles GET /api/credits/balance
func (h *CreditHandler) GetBalance(c fiber.Ctx) error {

	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	balance, err := h.creditService.GetBalance(userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to get balance",
		})
	}

	// -1 = unlimited for Pro tier
	monthlyLimit := 50
	if balance == -1 {
		monthlyLimit = -1
	}

	return c.Status(http.StatusOK).JSON(BalanceResponse{
		CreditsRemaining: balance,
		MonthlyLimit:     monthlyLimit,
		UsageThisMonth:   0, // Would calculate from transactions
	})
}

// GetHistory handles GET /api/credits/history?limit=20&offset=0
func (h *CreditHandler) GetHistory(c fiber.Ctx) error {

	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(map[string]string{
			"error": "Not authenticated",
		})
	}

	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	transactions, err := h.creditService.GetHistory(userID, limit, offset)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": "Failed to get history",
		})
	}

	var response []TransactionResponse
	for _, t := range transactions {
		response = append(response, TransactionResponse{
			ID:           t.ID,
			Amount:       t.Amount,
			Reason:       t.Reason,
			CampaignID:   t.CampaignID,
			BalanceAfter: t.BalanceAfter,
			CreatedAt:    t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return c.Status(http.StatusOK).JSON(map[string]interface{}{
		"transactions": response,
		"limit":        limit,
		"offset":       offset,
	})
}
