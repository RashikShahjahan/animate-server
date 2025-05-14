package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// setupRouter configures and returns the application router
func setupRouter() *mux.Router {
	r := mux.NewRouter()

	// Add global middlewares
	r.Use(corsMiddleware)
	r.Use(loggingMiddleware)

	// Public routes
	r.HandleFunc("/register", registerHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/login", loginHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/animation/{id}", getAnimationHandler).Methods(http.MethodGet)
	r.HandleFunc("/feed", getFeedHandler).Methods(http.MethodGet)

	// Create a subrouter for protected routes
	protected := r.PathPrefix("").Subrouter()
	protected.Use(authMiddleware)

	// Protected routes
	protected.HandleFunc("/generate-animation", animationHandler).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/save-animation", saveAnimationHandler).Methods(http.MethodPost, http.MethodOptions)

	return r
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logResponse("/register", "Invalid request format", err)
		encodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" || req.Username == "" {
		logResponse("/register", "Username, email and password are required", nil)
		encodeError(w, "Username, email and password are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	if userExists(req.Email) {
		logResponse("/register", "User already exists", nil)
		encodeError(w, "User already exists", http.StatusConflict)
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logResponse("/register", "Error hashing password", err)
		encodeError(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	// Create the user in the database
	userId, err := createUserWithUsername(req.Email, req.Username, string(hashedPassword))
	if err != nil {
		logResponse("/register", "Error creating user", err)
		encodeError(w, "Error creating user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	token, err := generateJWT(userId)
	if err != nil {
		logResponse("/register", "Error generating token", err)
		encodeError(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	logResponse("/register", "User registered successfully", nil)

	// Return the JWT token and user information
	response := RegisterResponse{
		Token: token,
		User: User{
			ID:       userId,
			Email:    req.Email,
			Username: req.Username,
		},
	}
	json.NewEncoder(w).Encode(response)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logResponse("/login", "Invalid request format", err)
		encodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" {
		logResponse("/login", "Email and password are required", nil)
		encodeError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Get user from database
	userId, storedHash, err := getUserCredentials(req.Email)
	if err != nil {
		logResponse("/login", "Invalid credentials", nil)
		encodeError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Compare password with stored hash
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password))
	if err != nil {
		logResponse("/login", "Invalid credentials", nil)
		encodeError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := generateJWT(userId)
	if err != nil {
		logResponse("/login", "Error generating token", err)
		encodeError(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Get user details
	user, err := getUserDetails(userId)
	if err != nil {
		logResponse("/login", "Error retrieving user details", err)
		encodeError(w, "Error retrieving user details", http.StatusInternalServerError)
		return
	}

	logResponse("/login", "User logged in successfully", nil)

	// Return the JWT token and user information
	response := LoginResponse{
		Token: token,
		User:  user,
	}
	json.NewEncoder(w).Encode(response)
}

// generateJWT creates a new JWT token for the given user ID
func generateJWT(userId string) (string, error) {
	// Get JWT secret key from environment variable
	secretKey := getAPIKey("JWT_SECRET_KEY")
	if secretKey == "" {
		return "", errors.New("JWT secret key not configured")
	}

	// Create a new token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userId,
		"exp":    time.Now().Add(time.Hour * 24 * 7).Unix(), // Token expires in 7 days
	})

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func animationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req AnimationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logResponse("/generate-animation", "Invalid request format", err)
		encodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Description == "" {
		logResponse("/generate-animation", "Description cannot be empty", nil)
		encodeError(w, "Description cannot be empty", http.StatusBadRequest)
		return
	}

	logRequest("/generate-animation", "Description: "+req.Description)

	// Get Claude API key from environment variable
	claudeAPIKey := getAPIKey("CLAUDE_API_KEY")
	if claudeAPIKey == "" {
		logResponse("/generate-animation", "Claude API key not configured", nil)
		encodeError(w, "Claude API key not configured", http.StatusInternalServerError)
		return
	}

	// Generate animation with Claude
	animation, err := generateAnimationWithClaude(req.Description, claudeAPIKey)
	if err != nil {
		logResponse("/generate-animation", "Error generating animation", err)
		encodeError(w, "Error generating animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sanitize the animation code by removing markdown fences
	animation = sanitizeSketchCode(animation)

	logResponse("/generate-animation", "Animation generated successfully", nil)

	// Return the animation code
	response := AnimationResponse{Code: animation}
	json.NewEncoder(w).Encode(response)
}

func saveAnimationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req SaveAnimationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logResponse("/save-animation", "Invalid request format", err)
		encodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	logRequest("/save-animation", "Received animation code to save")

	// Save the animation to the database
	id, err := saveAnimation(req.Code, req.Description)
	if err != nil {
		logResponse("/save-animation", "Error saving animation", err)
		encodeError(w, "Error saving animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logResponse("/save-animation", "Animation saved with ID: "+id, nil)

	// Return the animation ID
	response := SaveAnimationResponse{ID: id}
	json.NewEncoder(w).Encode(response)
}

func getAnimationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get animation ID from URL params
	vars := mux.Vars(r)
	id := vars["id"]

	logRequest("/animation/{id}", "Retrieving animation ID: "+id)

	// First check if the animation exists
	if !animationExists(id) {
		logResponse("/animation/{id}", "Animation not found with ID: "+id, nil)
		encodeError(w, "Animation not found", http.StatusNotFound)
		return
	}

	// Retrieve the animation from the database
	code, description, err := getAnimation(id)
	if err != nil {
		logResponse("/animation/{id}", "Error retrieving animation ID: "+id, err)
		// Always keep the Content-Type as application/json for consistent error handling
		encodeError(w, "Error retrieving animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logResponse("/animation/{id}", "Animation retrieved successfully", nil)

	// Return the animation code
	response := GetAnimationResponse{
		ID:          id,
		Code:        code,
		Description: description,
	}
	json.NewEncoder(w).Encode(response)
}

func getFeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	logRequest("/feed", "Retrieving random animation")

	// Retrieve a random animation from the database
	animation, err := getRandomAnimation()
	if err != nil {
		logResponse("/feed", "Error retrieving random animation", err)
		encodeError(w, "Error retrieving random animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logResponse("/feed", "Random animation retrieved successfully: "+animation.ID, nil)

	// Return the random animation
	json.NewEncoder(w).Encode(animation)
}
