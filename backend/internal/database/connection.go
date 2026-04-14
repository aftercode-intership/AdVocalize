package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

// New connects using individual parameters
func New(host, port, user, password, dbname string) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	return connect(dsn)
}

// NewFromURL connects using a full postgres:// URL — use this instead
func NewFromURL(databaseURL string) (*DB, error) {
	return connect(databaseURL)
}

func connect(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)

	log.Println("Database connected successfully")
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) GetConnection() *sql.DB {
	return d.conn
}