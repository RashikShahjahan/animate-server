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
	log.Println("[DB] Reading init_db.sql script...")

	// Check if the init SQL file exists
	if _, err := os.Stat("init_db.sql"); os.IsNotExist(err) {
		log.Println("[DB] init_db.sql file not found, skipping initialization script")
		return nil
	}

	// Read the init SQL file
	sqlBytes, err := os.ReadFile("init_db.sql")
	if err != nil {
		return fmt.Errorf("failed to read init_db.sql: %v", err)
	}

	if len(sqlBytes) == 0 {
		log.Println("[DB] init_db.sql file is empty, skipping initialization script")
		return nil
	}

	sqlContent := string(sqlBytes)
	log.Printf("[DB] Loaded init_db.sql (%d bytes)", len(sqlBytes))

	log.Println("[DB] Starting transaction for init script execution...")
	// Execute the entire script as a single transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer func() {
		if err != nil {
			log.Println("[DB] Rolling back transaction due to error")
			tx.Rollback()
		}
	}()

	// Execute the SQL script directly
	log.Println("[DB] Executing SQL script...")
	_, err = tx.Exec(sqlContent)
	if err != nil {
		return fmt.Errorf("failed to execute SQL script: %v", err)
	}

	log.Println("[DB] Committing transaction...")
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Println("[DB] Init script executed successfully")
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

// getRandomAnimation retrieves a single random animation from the database
func getRandomAnimation() (GetAnimationResponse, error) {
	log.Printf("[DB] Retrieving random animation")

	// Query to get a random animation
	var id string
	var code string
	var description sql.NullString

	// PostgreSQL's RANDOM() function to select a random row
	err := db.QueryRow("SELECT id, code, description FROM animations ORDER BY RANDOM() LIMIT 1").Scan(&id, &code, &description)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[DB ERROR] No animations found in database")
			return GetAnimationResponse{}, errors.New("no animations found")
		}
		log.Printf("[DB ERROR] Failed to retrieve random animation: %v", err)
		return GetAnimationResponse{}, err
	}

	// Handle NULL description
	descriptionValue := ""
	if description.Valid {
		descriptionValue = description.String
	}

	// Create response
	animation := GetAnimationResponse{
		ID:          id,
		Code:        code,
		Description: descriptionValue,
	}

	log.Printf("[DB] Random animation retrieved successfully with ID: %s", id)
	return animation, nil
}

// saveMood stores a user's mood reaction to an animation
func saveMood(userId string, animationId string, mood string) error {
	if userId == "" || animationId == "" || mood == "" {
		log.Printf("[DB ERROR] Cannot save mood with empty user ID, animation ID, or mood value")
		return errors.New("user ID, animation ID, and mood cannot be empty")
	}

	log.Printf("[DB] Saving mood for user ID: %s, animation ID: %s, mood: %s", userId, animationId, mood)

	// Store the mood in the database
	_, err := db.Exec("INSERT INTO user_moods (user_id, animation_id, mood) VALUES ($1, $2, $3)",
		userId, animationId, mood)
	if err != nil {
		log.Printf("[DB ERROR] Failed to save mood to database: %v", err)
		return err
	}

	log.Printf("[DB] Mood saved successfully for user ID: %s, animation ID: %s", userId, animationId)
	return nil
}

// performDatabaseMigrations runs any necessary database migrations
func performDatabaseMigrations() error {
	log.Println("[DB] Checking for necessary database migrations...")

	// Add username column to users table if it doesn't exist
	_, err := db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'username'
			) THEN
				ALTER TABLE users ADD COLUMN username VARCHAR(255);
				RAISE NOTICE 'Added username column to users table';
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("failed to check/add username column: %v", err)
	}

	// Check index existence on user_moods (for migrations from older versions)
	var indexExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'user_moods' AND indexname = 'idx_user_moods_user_id'
		)
	`).Scan(&indexExists)

	if err != nil {
		log.Printf("[DB] Warning: Failed to check index existence: %v", err)
	} else if !indexExists {
		log.Println("[DB] Creating missing indexes on user_moods table...")
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_moods_user_id ON user_moods(user_id)`)
		if err != nil {
			log.Printf("[DB] Warning: Failed to create user_id index: %v", err)
		}

		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_moods_animation_id ON user_moods(animation_id)`)
		if err != nil {
			log.Printf("[DB] Warning: Failed to create animation_id index: %v", err)
		}
	}

	return nil
}
