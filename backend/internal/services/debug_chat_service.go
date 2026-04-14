package services

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Add this function to your ChatService to help debug
func (cs *ChatService) DebugCreateSession(ctx context.Context, userID, topic string) error {
	log.Printf("[DEBUG] CreateSession called with userID=%s, topic=%s", userID, topic)

	query := `
		INSERT INTO chat_sessions (id, user_id, topic, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, topic, status, created_at, updated_at
	`

	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())

	log.Printf("[DEBUG] Executing query: %s", query)
	log.Printf("[DEBUG] With params: id=%s, user_id=%s, topic=%s", sessionID, userID, topic)

	var returnedID, returnedUserID, returnedTopic, returnedStatus string
	var createdAt, updatedAt time.Time

	err := cs.db.QueryRowContext(
		ctx,
		query,
		sessionID,
		userID,
		topic,
		"active",
		time.Now(),
		time.Now(),
	).Scan(
		&returnedID,
		&returnedUserID,
		&returnedTopic,
		&returnedStatus,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		log.Printf("[DEBUG] Query failed: %v", err)
		log.Printf("[DEBUG] This likely means the chat_sessions table doesn't exist or has different schema")
		return err
	}

	log.Printf("[DEBUG] Session created successfully: %s", returnedID)
	return nil
}

// Test the database connection
func (cs *ChatService) TestDatabaseConnection() error {
	log.Println("[DEBUG] Testing database connection...")

	err := cs.db.Ping()
	if err != nil {
		log.Printf("[DEBUG] Database ping failed: %v", err)
		return err
	}

	log.Println("[DEBUG] Database connection OK")

	// List all tables
	rows, err := cs.db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`)
	if err != nil {
		log.Printf("[DEBUG] Failed to list tables: %v", err)
		return err
	}
	defer rows.Close()

	log.Println("[DEBUG] Tables in database:")
	for rows.Next() {
		var tableName string
		rows.Scan(&tableName)
		log.Printf("[DEBUG]   - %s", tableName)
	}

	return nil
}

// Check if a specific table exists
func (cs *ChatService) TableExists(tableName string) (bool, error) {
	log.Printf("[DEBUG] Checking if table '%s' exists...", tableName)

	query := `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)
	`

	var exists bool
	err := cs.db.QueryRow(query, tableName).Scan(&exists)
	if err != nil {
		log.Printf("[DEBUG] Error checking table: %v", err)
		return false, err
	}

	if exists {
		log.Printf("[DEBUG] ✓ Table '%s' exists", tableName)
	} else {
		log.Printf("[DEBUG] ✗ Table '%s' NOT found", tableName)
	}

	return exists, nil
}

// Get the schema of a table
func (cs *ChatService) GetTableSchema(tableName string) error {
	log.Printf("[DEBUG] Getting schema for table '%s'...", tableName)

	query := `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := cs.db.Query(query, tableName)
	if err != nil {
		log.Printf("[DEBUG] Failed to get schema: %v", err)
		return err
	}
	defer rows.Close()

	log.Printf("[DEBUG] Columns in table '%s':", tableName)
	for rows.Next() {
		var colName, dataType string
		var nullable string
		rows.Scan(&colName, &dataType, &nullable)
		log.Printf("[DEBUG]   - %s (%s, nullable: %s)", colName, dataType, nullable)
	}

	return nil
}