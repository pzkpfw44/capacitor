package models

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"wave_capacitor/config"

	_ "github.com/lib/pq" // PostgreSQL driver for CockroachDB
)

// Global database instance
var db *sql.DB

// User represents a user in the system
type User struct {
	ID               int    `json:"-"`
	Username         string `json:"username"`
	PublicKey        string `json:"public_key"`
	EncryptedPrivKey string `json:"encrypted_private_key"`
}

// InitializeDB connects to CockroachDB and sets up required tables
func InitializeDB() error {
	connStr := config.GetDBConnectionString()
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("database connection test failed: %v", err)
	}
	log.Println("✅ Connected to database successfully")

	// Create users table if it doesn't exist
	createUsersTable := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			public_key TEXT NOT NULL,
			encrypted_private_key TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(createUsersTable); err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}
	log.Println("✅ Users table ready")

	return nil
}

// CreateUser stores a new user in the database
func CreateUser(username string, publicKey []byte, encryptedPrivateKey []byte) error {
	if db == nil {
		return errors.New("database connection not initialized")
	}

	// Convert binary data to base64 strings for storage
	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKey)
	
	// For encrypted private key, check if it's already a valid JSON string
	var encPrivKeyStr string
	if json.Valid(encryptedPrivateKey) {
		encPrivKeyStr = string(encryptedPrivateKey)
	} else {
		// If not valid JSON, store as base64 string
		encPrivKeyStr = base64.StdEncoding.EncodeToString(encryptedPrivateKey)
	}

	// Insert the user
	query := `INSERT INTO users (username, public_key, encrypted_private_key) VALUES ($1, $2, $3)`
	_, err := db.Exec(query, username, publicKeyBase64, encPrivKeyStr)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	log.Printf("✅ User '%s' created successfully", username)
	return nil
}

// GetUser retrieves a user by username
func GetUser(username string) (*User, error) {
	if db == nil {
		return nil, errors.New("database connection not initialized")
	}

	var user User
	query := `SELECT id, username, public_key, encrypted_private_key FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.PublicKey, &user.EncryptedPrivKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user '%s' not found", username)
		}
		return nil, fmt.Errorf("error retrieving user: %v", err)
	}

	return &user, nil
}

// UpdateUserKeys updates the public key and encrypted private key for a user
func UpdateUserKeys(username, publicKey string, encryptedPrivateKey interface{}) error {
	if db == nil {
		return errors.New("database connection not initialized")
	}

	var encPrivKeyStr string
	switch v := encryptedPrivateKey.(type) {
	case string:
		encPrivKeyStr = v
	case map[string]interface{}:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal encrypted private key: %v", err)
		}
		encPrivKeyStr = string(jsonBytes)
	default:
		return errors.New("invalid encrypted private key format")
	}

	// Update the user's keys
	query := `UPDATE users SET public_key = $1, encrypted_private_key = $2, updated_at = CURRENT_TIMESTAMP WHERE username = $3`
	result, err := db.Exec(query, publicKey, encPrivKeyStr, username)
	if err != nil {
		return fmt.Errorf("failed to update user keys: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %v", err)
	}

	if rowsAffected == 0 {
		// If no rows were updated, create a new user
		insertQuery := `INSERT INTO users (username, public_key, encrypted_private_key) VALUES ($1, $2, $3)`
		_, err := db.Exec(insertQuery, username, publicKey, encPrivKeyStr)
		if err != nil {
			return fmt.Errorf("failed to create user during key update: %v", err)
		}
		log.Printf("✅ Created new user '%s' during key update", username)
		return nil
	}

	log.Printf("✅ Updated keys for user '%s'", username)
	return nil
}

// DeleteUser removes a user from the database
func DeleteUser(username string) error {
	if db == nil {
		return errors.New("database connection not initialized")
	}

	query := `DELETE FROM users WHERE username = $1`
	result, err := db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found for deletion", username)
	}

	log.Printf("✅ Deleted user '%s'", username)
	return nil
}

// UserExists checks if a username already exists in the database
func UserExists(username string) (bool, error) {
	if db == nil {
		return false, errors.New("database connection not initialized")
	}

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	err := db.QueryRow(query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if user exists: %v", err)
	}

	return exists, nil
}
