// backend/internal/services/credit_service.go
package services

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"log"
)

type CreditService struct {
	db *sql.DB
}

func NewCreditService(db *sql.DB) *CreditService {
	return &CreditService{db: db}
}

// Transaction represents a credit transaction
type Transaction struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Amount       int       `json:"amount"`
	Reason       string    `json:"reason"`
	CampaignID   string    `json:"campaign_id,omitempty"`
	BalanceAfter int       `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

// DeductCredits atomically deducts credits from user
// This ensures no double-charging even with concurrent requests
func (cs *CreditService) DeductCredits(userID string, amount int, reason string, campaignID *string) error {
	// Start transaction
	tx, err := cs.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock the user row (prevents race conditions)
	var currentBalance int
	query := `
    SELECT credits_remaining
    FROM users
    WHERE id = $1
    FOR UPDATE
  `

	err = tx.QueryRow(query, userID).Scan(&currentBalance)
	if err != nil {
		return fmt.Errorf("failed to get user balance: %w", err)
	}

	// Check if sufficient credits
	if currentBalance >= 0 && currentBalance < amount { // -1 = unlimited (Pro tier)
		return fmt.Errorf("insufficient credits: have %d, need %d", currentBalance, amount)
	}

	// Calculate new balance
	var newBalance int
	if currentBalance == -1 { // Unlimited
		newBalance = -1
	} else {
		newBalance = currentBalance - amount
	}

	// Update user credits
	updateQuery := `
    UPDATE users
    SET credits_remaining = $1, updated_at = CURRENT_TIMESTAMP
    WHERE id = $2
  `

	_, err = tx.Exec(updateQuery, newBalance, userID)
	if err != nil {
		return fmt.Errorf("failed to update credits: %w", err)
	}

	// Log transaction
	transactionID := uuid.New().String()
	transactionQuery := `
    INSERT INTO credit_transactions (id, user_id, amount, reason, campaign_id, balance_after, created_at)
    VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
  `

	_, err = tx.Exec(
		transactionQuery,
		transactionID,
		userID,
		-amount, // Negative for deduction
		reason,
		campaignID,
		newBalance,
	)

	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Credits deducted: user=%s, amount=%d, reason=%s, new_balance=%d",
		userID, amount, reason, newBalance)

	return nil
}

// AddCredits atomically adds credits to user
func (cs *CreditService) AddCredits(userID string, amount int, reason string) error {
	tx, err := cs.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock row
	var currentBalance int
	err = tx.QueryRow(`
    SELECT credits_remaining FROM users WHERE id = $1 FOR UPDATE
  `, userID).Scan(&currentBalance)

	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// Calculate new balance
	var newBalance int
	if currentBalance == -1 { // Unlimited
		newBalance = -1
	} else {
		newBalance = currentBalance + amount
	}

	// Update
	_, err = tx.Exec(`
    UPDATE users SET credits_remaining = $1, updated_at = CURRENT_TIMESTAMP
    WHERE id = $2
  `, newBalance, userID)

	if err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	// Log
	_, err = tx.Exec(`
    INSERT INTO credit_transactions (id, user_id, amount, reason, balance_after, created_at)
    VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
  `, uuid.New().String(), userID, amount, reason, newBalance)

	if err != nil {
		return fmt.Errorf("failed to log: %w", err)
	}

	return tx.Commit()
}

// GetBalance returns current credit balance
func (cs *CreditService) GetBalance(userID string) (int, error) {
	var balance int
	err := cs.db.QueryRow(`
    SELECT credits_remaining FROM users WHERE id = $1
  `, userID).Scan(&balance)

	return balance, err
}

// GetHistory returns credit transaction history
func (cs *CreditService) GetHistory(userID string, limit int, offset int) ([]Transaction, error) {
	rows, err := cs.db.Query(`
    SELECT id, user_id, amount, reason, campaign_id, balance_after, created_at
    FROM credit_transactions
    WHERE user_id = $1
    ORDER BY created_at DESC
    LIMIT $2 OFFSET $3
  `, userID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var campaignID sql.NullString
		err := rows.Scan(
			&t.ID, &t.UserID, &t.Amount, &t.Reason, &campaignID, &t.BalanceAfter, &t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		if campaignID.Valid {
			t.CampaignID = campaignID.String
		}
		transactions = append(transactions, t)
	}

	return transactions, nil
}

// MonthlyReset resets free tier credits (runs daily, idempotent)
func (cs *CreditService) MonthlyReset() error {
	// Only reset on first of month
	if time.Now().Day() != 1 {
		return nil
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin: %w", err)
	}
	defer tx.Rollback()

	// Reset FREE tier users to 50 credits
	result, err := tx.Exec(`
    UPDATE users
    SET credits_remaining = 50, updated_at = CURRENT_TIMESTAMP
    WHERE subscription_tier = 'FREE' AND credits_remaining < 50
  `)

	if err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("Monthly credit reset: %d users reset to 50 credits", rows)

	return tx.Commit()
}
