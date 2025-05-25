package internal

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

// InitDB initializes the PostgreSQL database connection
func InitDB() error {
	log.Println("[DB] Initializing database connection...")

	// Load environment variables from .env file if they haven't been loaded yet
	if os.Getenv("DB_HOST") == "" && os.Getenv("DB_USER") == "" && os.Getenv("DB_PASSWORD") == "" {
		log.Println("[DB] Environment variables not found, attempting to load from .env file")
		if err := godotenv.Load(); err != nil {
			log.Println("[DB] Warning: .env file not found or could not be loaded")
		}
	}

	// Get PostgreSQL connection string from environment variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Set defaults if environment variables are not set
	if dbHost == "" {
		dbHost = "localhost"
		log.Println("[DB] Using default host: localhost")
	}
	if dbPort == "" {
		dbPort = "5432"
		log.Println("[DB] Using default port: 5432")
	}
	if dbName == "" {
		dbName = "animations"
		log.Println("[DB] Using default database name: animations")
	}

	log.Printf("[DB] Connecting to PostgreSQL at %s:%s", dbHost, dbPort)

	// First, connect to the 'postgres' database to check if our target database exists
	connStrPostgres := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword)

	dbPostgres, err := sql.Open("postgres", connStrPostgres)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %v", err)
	}
	defer dbPostgres.Close()

	// Check if we can connect
	if err = dbPostgres.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres database: %v", err)
	}
	log.Println("[DB] Successfully connected to PostgreSQL")

	// Check if our database exists
	var exists bool
	err = dbPostgres.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %v", err)
	}

	// If database doesn't exist, create it
	if !exists {
		log.Printf("[DB] Database '%s' does not exist, creating it...", dbName)
		_, err = dbPostgres.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return fmt.Errorf("failed to create database: %v", err)
		}
		log.Printf("[DB] Database '%s' created successfully", dbName)
	} else {
		log.Printf("[DB] Database '%s' already exists", dbName)
	}

	// Now connect to our target database
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to the PostgreSQL database
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s database: %v", dbName, err)
	}

	// Check the connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping %s database: %v", dbName, err)
	}
	log.Printf("[DB] Successfully connected to '%s' database", dbName)

	// Create tables
	log.Println("[DB] Setting up database tables...")

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
		return fmt.Errorf("failed to create animations table: %v", err)
	}
	log.Println("[DB] Animations table created or already exists")

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
		return fmt.Errorf("failed to create users table: %v", err)
	}
	log.Println("[DB] Users table created or already exists")

	// Create user_moods table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_moods (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(32) NOT NULL,
			animation_id VARCHAR(32) NOT NULL,
			mood VARCHAR(20) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (animation_id) REFERENCES animations(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_moods table: %v", err)
	}
	log.Println("[DB] User_moods table created or already exists")

	// Create indexes for better query performance
	log.Println("[DB] Creating indexes...")

	// Add index on animations table for faster lookups
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_animations_id ON animations(id)`)
	if err != nil {
		log.Printf("[DB] Warning: Failed to create index on animations table: %v", err)
	}

	// Add indexes on user_moods table for faster lookups
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_moods_user_id ON user_moods(user_id)`)
	if err != nil {
		log.Printf("[DB] Warning: Failed to create user_id index on user_moods table: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_moods_animation_id ON user_moods(animation_id)`)
	if err != nil {
		log.Printf("[DB] Warning: Failed to create animation_id index on user_moods table: %v", err)
	}

	// Add index on email for faster user lookups
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`)
	if err != nil {
		log.Printf("[DB] Warning: Failed to create email index on users table: %v", err)
	}

	// Perform any necessary migrations for existing databases
	log.Println("[DB] Checking for necessary database migrations...")
	if err := performDatabaseMigrations(); err != nil {
		log.Printf("[DB] Warning: Some database migrations may have failed: %v", err)
	}

	// Execute init SQL script
	log.Println("[DB] Executing initialization SQL script...")
	err = executeInitScript()
	if err != nil {
		log.Printf("[DB] Warning: Failed to execute init SQL script: %v", err)
	} else {
		log.Println("[DB] Init SQL script executed successfully")
	}

	log.Println("[DB] Database initialization completed successfully")
	return nil
}

// executeInitScript reads and executes the init_db.sql script
func executeInitScript() error {
	// Read the SQL script file
	sqlBytes, err := os.ReadFile("init_db.sql")
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("[DB] init_db.sql file not found, skipping initialization script")
			return nil
		}
		return fmt.Errorf("failed to read init_db.sql: %v", err)
	}

	// Convert to string and split by semicolons to get individual statements
	sqlScript := string(sqlBytes)
	statements := strings.Split(sqlScript, ";")

	// Execute each statement
	for i, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		log.Printf("[DB] Executing statement %d...", i+1)
		_, err := db.Exec(statement)
		if err != nil {
			log.Printf("[DB] Warning: Failed to execute statement %d: %v", i+1, err)
			log.Printf("[DB] Statement was: %s", statement)
			// Continue with other statements even if one fails
		}
	}

	return nil
}

// generateRandomID generates a random ID for database records
func generateRandomID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:22], nil
}

// UserExists checks if a user with the given email already exists
func UserExists(email string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&count)
	if err != nil {
		log.Printf("[DB ERROR] Failed to check if user exists: %v", err)
		return false
	}
	return count > 0
}

// CreateUserWithUsername creates a new user with username in the database
func CreateUserWithUsername(email, username, passwordHash string) (string, error) {
	// Generate a random user ID
	userId, err := generateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate user ID: %v", err)
	}

	// Insert the user into the database
	_, err = db.Exec(
		"INSERT INTO users (id, email, username, password_hash) VALUES ($1, $2, $3, $4)",
		userId, email, username, passwordHash,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert user: %v", err)
	}

	log.Printf("[DB] User created successfully with ID: %s", userId)
	return userId, nil
}

// GetUserCredentials retrieves user credentials for authentication
func GetUserCredentials(email string) (string, string, error) {
	var userId, passwordHash string
	err := db.QueryRow(
		"SELECT id, password_hash FROM users WHERE email = $1",
		email,
	).Scan(&userId, &passwordHash)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", errors.New("user not found")
		}
		return "", "", fmt.Errorf("database error: %v", err)
	}

	return userId, passwordHash, nil
}

// SaveAnimation saves an animation to the database
func SaveAnimation(code string, description string) (string, error) {
	// Generate a random animation ID
	animationId, err := generateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate animation ID: %v", err)
	}

	// Insert the animation into the database
	_, err = db.Exec(
		"INSERT INTO animations (id, code, description) VALUES ($1, $2, $3)",
		animationId, code, description,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert animation: %v", err)
	}

	log.Printf("[DB] Animation saved successfully with ID: %s", animationId)
	return animationId, nil
}

// GetAnimation retrieves an animation from the database
func GetAnimation(id string) (string, string, error) {
	var code, description string
	err := db.QueryRow(
		"SELECT code, description FROM animations WHERE id = $1",
		id,
	).Scan(&code, &description)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", errors.New("animation not found")
		}
		return "", "", fmt.Errorf("database error: %v", err)
	}

	return code, description, nil
}

// GetUserDetails retrieves user details by user ID
func GetUserDetails(userId string) (User, error) {
	var user User
	err := db.QueryRow(
		"SELECT id, email, username FROM users WHERE id = $1",
		userId,
	).Scan(&user.ID, &user.Email, &user.Username)

	if err != nil {
		if err == sql.ErrNoRows {
			return user, errors.New("user not found")
		}
		return user, fmt.Errorf("database error: %v", err)
	}

	return user, nil
}

// AnimationExists checks if an animation with the given ID exists
func AnimationExists(id string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM animations WHERE id = $1", id).Scan(&count)
	if err != nil {
		log.Printf("[DB ERROR] Failed to check if animation exists: %v", err)
		return false
	}
	return count > 0
}

// GetRandomAnimation retrieves a random animation from the database
func GetRandomAnimation() (GetAnimationResponse, error) {
	var animation GetAnimationResponse
	err := db.QueryRow(
		"SELECT id, code, description FROM animations ORDER BY RANDOM() LIMIT 1",
	).Scan(&animation.ID, &animation.Code, &animation.Description)

	if err != nil {
		if err == sql.ErrNoRows {
			return animation, errors.New("no animations found")
		}
		return animation, fmt.Errorf("database error: %v", err)
	}

	return animation, nil
}

// SaveMood saves a user's mood for an animation
func SaveMood(userId string, animationId string, mood string) error {
	// Insert the mood into the database
	_, err := db.Exec(
		"INSERT INTO user_moods (user_id, animation_id, mood) VALUES ($1, $2, $3)",
		userId, animationId, mood,
	)
	if err != nil {
		return fmt.Errorf("failed to insert mood: %v", err)
	}

	log.Printf("[DB] Mood saved successfully for user %s and animation %s", userId, animationId)
	return nil
}

// performDatabaseMigrations performs any necessary database migrations
func performDatabaseMigrations() error {
	// Check if username column exists in users table
	var columnExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'users' 
			AND column_name = 'username'
		)
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check for username column: %v", err)
	}

	// Add username column if it doesn't exist
	if !columnExists {
		log.Println("[DB] Adding username column to users table...")
		_, err = db.Exec("ALTER TABLE users ADD COLUMN username VARCHAR(255)")
		if err != nil {
			return fmt.Errorf("failed to add username column: %v", err)
		}
		log.Println("[DB] Username column added successfully")
	}

	return nil
}
