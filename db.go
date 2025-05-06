package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

// initDB initializes the PostgreSQL database connection
func initDB() error {
	// Load environment variables from .env file if they haven't been loaded yet
	if os.Getenv("DB_HOST") == "" && os.Getenv("DB_USER") == "" && os.Getenv("DB_PASSWORD") == "" {
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: .env file not found or could not be loaded")
		}
	}

	// Get PostgreSQL connection string from environment variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
	if dbName == "" {
		dbName = "animations"
	}

	// First, connect to the 'postgres' database to check if our target database exists
	connStrPostgres := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword)

	dbPostgres, err := sql.Open("postgres", connStrPostgres)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %v", err)
	}
	defer dbPostgres.Close()

	// Check if our database exists
	var exists bool
	err = dbPostgres.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %v", err)
	}

	// If database doesn't exist, create it
	if !exists {
		log.Printf("Database '%s' does not exist, creating it...", dbName)
		_, err = dbPostgres.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return fmt.Errorf("failed to create database: %v", err)
		}
		log.Printf("Database '%s' created successfully", dbName)
	}

	// Now connect to our target database
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to the PostgreSQL database
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	// Check the connection
	if err = db.Ping(); err != nil {
		return err
	}

	// Create animations table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS animations (
			id VARCHAR(32) PRIMARY KEY,
			code TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Execute init SQL script
	err = executeInitScript()
	if err != nil {
		log.Printf("Warning: Failed to execute init SQL script: %v", err)
	} else {
		log.Println("Init SQL script executed successfully")
	}

	return nil
}

// executeInitScript reads and executes the init_db.sql script
func executeInitScript() error {
	// Read the init SQL file
	sqlBytes, err := os.ReadFile("init_db.sql")
	if err != nil {
		return fmt.Errorf("failed to read init_db.sql: %v", err)
	}

	sqlContent := string(sqlBytes)

	// Execute the entire script as a single transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Execute the SQL script directly
	_, err = tx.Exec(sqlContent)
	if err != nil {
		return fmt.Errorf("failed to execute SQL script: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// saveAnimation stores an animation code in the database and returns its ID
func saveAnimation(code string, description string) (string, error) {
	if code == "" {
		log.Printf("[DB ERROR] Cannot save empty animation code")
		return "", errors.New("animation code cannot be empty")
	}

	// Generate a random ID
	idBytes := make([]byte, 8)
	_, err := rand.Read(idBytes)
	if err != nil {
		log.Printf("[DB ERROR] Failed to generate random ID: %v", err)
		return "", err
	}
	id := base64.URLEncoding.EncodeToString(idBytes)

	log.Printf("[DB] Saving animation with ID: %s", id)

	// Store the animation in the database
	_, err = db.Exec("INSERT INTO animations (id, code, description) VALUES ($1, $2, $3)", id, code, description)
	if err != nil {
		log.Printf("[DB ERROR] Failed to save animation to database: %v", err)
		return "", err
	}

	log.Printf("[DB] Animation saved successfully with ID: %s", id)
	return id, nil
}

// getAnimation retrieves an animation by ID from the database
func getAnimation(id string) (string, string, error) {
	if id == "" {
		log.Printf("[DB ERROR] Cannot retrieve animation with empty ID")
		return "", "", errors.New("animation ID cannot be empty")
	}

	log.Printf("[DB] Retrieving animation with ID: %s", id)

	// Retrieve the animation from the database
	var code string
	var description sql.NullString
	err := db.QueryRow("SELECT code, description FROM animations WHERE id = $1", id).Scan(&code, &description)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[DB ERROR] Animation not found with ID: %s", id)
			return "", "", errors.New("animation not found")
		}
		log.Printf("[DB ERROR] Failed to retrieve animation from database: %v", err)
		return "", "", err
	}

	// Handle NULL description
	descriptionValue := ""
	if description.Valid {
		descriptionValue = description.String
	}

	log.Printf("[DB] Animation retrieved successfully with ID: %s", id)
	return code, descriptionValue, nil
}
