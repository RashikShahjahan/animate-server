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

		// Special case for migration: Check if users table exists but doesn't have username column
		_, err = dbPostgres.Exec(fmt.Sprintf(`
			DO $$
			BEGIN
				IF EXISTS (
					SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'users'
				) AND NOT EXISTS (
					SELECT 1 FROM information_schema.columns 
					WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'username'
				) THEN
					-- Add username column to users table
					ALTER TABLE users ADD COLUMN username VARCHAR(255);
				END IF;
			END
			$$;
		`))
		if err != nil {
			log.Printf("Warning: Could not check or perform migration: %v", err)
		}
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

	// Create users table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(32) PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(255),
			password_hash TEXT NOT NULL,
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

// userExists checks if a user with the given email already exists
func userExists(email string) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		log.Printf("[DB ERROR] Failed to check if user exists: %v", err)
		return false
	}
	return exists
}

// createUserWithUsername stores a new user with username in the database and returns the user ID
func createUserWithUsername(email, username, passwordHash string) (string, error) {
	if email == "" || passwordHash == "" {
		log.Printf("[DB ERROR] Cannot create user with empty email or password")
		return "", errors.New("email and password hash cannot be empty")
	}

	// Generate a random ID
	idBytes := make([]byte, 8)
	_, err := rand.Read(idBytes)
	if err != nil {
		log.Printf("[DB ERROR] Failed to generate random ID: %v", err)
		return "", err
	}
	id := base64.URLEncoding.EncodeToString(idBytes)

	log.Printf("[DB] Creating user with ID: %s", id)

	// Store the user in the database
	_, err = db.Exec("INSERT INTO users (id, email, username, password_hash) VALUES ($1, $2, $3, $4)",
		id, email, username, passwordHash)
	if err != nil {
		log.Printf("[DB ERROR] Failed to create user: %v", err)
		return "", err
	}

	log.Printf("[DB] User created successfully with ID: %s", id)
	return id, nil
}

// getUserCredentials retrieves a user's ID and password hash by email
func getUserCredentials(email string) (string, string, error) {
	if email == "" {
		log.Printf("[DB ERROR] Cannot retrieve user with empty email")
		return "", "", errors.New("email cannot be empty")
	}

	log.Printf("[DB] Retrieving user credentials for email: %s", email)

	// Retrieve the user from the database
	var id string
	var passwordHash string
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE email = $1", email).Scan(&id, &passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[DB ERROR] User not found with email: %s", email)
			return "", "", errors.New("user not found")
		}
		log.Printf("[DB ERROR] Failed to retrieve user from database: %v", err)
		return "", "", err
	}

	return id, passwordHash, nil
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

// getUserDetails retrieves user details by ID
func getUserDetails(userId string) (User, error) {
	if userId == "" {
		log.Printf("[DB ERROR] Cannot retrieve user with empty ID")
		return User{}, errors.New("user ID cannot be empty")
	}

	log.Printf("[DB] Retrieving user details for ID: %s", userId)

	// Retrieve the user from the database
	var user User
	err := db.QueryRow("SELECT id, username, email FROM users WHERE id = $1", userId).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[DB ERROR] User not found with ID: %s", userId)
			return User{}, errors.New("user not found")
		}
		log.Printf("[DB ERROR] Failed to retrieve user from database: %v", err)
		return User{}, err
	}

	return user, nil
}

// animationExists checks if an animation with the given ID exists
func animationExists(id string) bool {
	if id == "" {
		return false
	}

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM animations WHERE id = $1)", id).Scan(&exists)
	if err != nil {
		log.Printf("[DB ERROR] Failed to check if animation exists: %v", err)
		return false
	}
	return exists
}
