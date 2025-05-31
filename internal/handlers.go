package internal

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// SetupRouter configures and returns the application router
func SetupRouter() *mux.Router {
	r := mux.NewRouter()

	// Add global middlewares
	r.Use(CorsMiddleware)
	r.Use(LoggingMiddleware)

	// Public routes
	r.HandleFunc("/register", registerHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/login", loginHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/animation/{id}", getAnimationHandler).Methods(http.MethodGet)
	r.HandleFunc("/feed", getFeedHandler).Methods(http.MethodGet)

	// Create a subrouter for protected routes
	protected := r.PathPrefix("").Subrouter()
	protected.Use(AuthMiddleware)

	// Protected routes
	protected.HandleFunc("/generate-animation", animationHandler).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/save-animation", saveAnimationHandler).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/save-mood", saveMoodHandler).Methods(http.MethodPost, http.MethodOptions)

	return r
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogResponse("/register", "Invalid request format", err)
		EncodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" || req.Username == "" {
		LogResponse("/register", "Username, email and password are required", nil)
		EncodeError(w, "Username, email and password are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	if UserExists(req.Email) {
		LogResponse("/register", "User already exists", nil)
		EncodeError(w, "User already exists", http.StatusConflict)
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		LogResponse("/register", "Error hashing password", err)
		EncodeError(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	// Create the user in the database
	userId, err := CreateUserWithUsername(req.Email, req.Username, string(hashedPassword))
	if err != nil {
		LogResponse("/register", "Error creating user", err)
		EncodeError(w, "Error creating user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	token, err := generateJWT(userId)
	if err != nil {
		LogResponse("/register", "Error generating token", err)
		EncodeError(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	LogResponse("/register", "User registered successfully", nil)

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
		LogResponse("/login", "Invalid request format", err)
		EncodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" {
		LogResponse("/login", "Email and password are required", nil)
		EncodeError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Get user from database
	userId, storedHash, err := GetUserCredentials(req.Email)
	if err != nil {
		LogResponse("/login", "Invalid credentials", nil)
		EncodeError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Compare password with stored hash
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password))
	if err != nil {
		LogResponse("/login", "Invalid credentials", nil)
		EncodeError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := generateJWT(userId)
	if err != nil {
		LogResponse("/login", "Error generating token", err)
		EncodeError(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Get user details
	user, err := GetUserDetails(userId)
	if err != nil {
		LogResponse("/login", "Error retrieving user details", err)
		EncodeError(w, "Error retrieving user details", http.StatusInternalServerError)
		return
	}

	LogResponse("/login", "User logged in successfully", nil)

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
	secretKey := GetAPIKey("JWT_SECRET_KEY")
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
		LogResponse("/generate-animation", "Invalid request format", err)
		EncodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Description == "" {
		LogResponse("/generate-animation", "Description cannot be empty", nil)
		EncodeError(w, "Description cannot be empty", http.StatusBadRequest)
		return
	}

	LogRequest("/generate-animation", "Description: "+req.Description)

	// Get Claude API key from environment variable
	claudeAPIKey := GetAPIKey("CLAUDE_API_KEY")
	if claudeAPIKey == "" {
		LogResponse("/generate-animation", "Claude API key not configured", nil)
		EncodeError(w, "Claude API key not configured", http.StatusInternalServerError)
		return
	}

	// Generate animation with Claude
	animation, err := GenerateAnimationWithClaude(req.Description, claudeAPIKey)
	if err != nil {
		LogResponse("/generate-animation", "Error generating animation", err)
		EncodeError(w, "Error generating animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sanitize the animation code by removing markdown fences
	animation = SanitizeAnimationCode(animation)

	LogResponse("/generate-animation", "Animation generated successfully", nil)

	// Return the animation code
	response := AnimationResponse{Code: animation}
	json.NewEncoder(w).Encode(response)
}

func saveAnimationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req SaveAnimationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogResponse("/save-animation", "Invalid request format", err)
		EncodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	LogRequest("/save-animation", "Received animation code to save")

	// Save the animation to the database
	id, err := SaveAnimation(req.Code, req.Description)
	if err != nil {
		LogResponse("/save-animation", "Error saving animation", err)
		EncodeError(w, "Error saving animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	LogResponse("/save-animation", "Animation saved with ID: "+id, nil)

	// Return the animation ID
	response := SaveAnimationResponse{ID: id}
	json.NewEncoder(w).Encode(response)
}

func getAnimationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get animation ID from URL params
	vars := mux.Vars(r)
	id := vars["id"]

	LogRequest("/animation/{id}", "Retrieving animation ID: "+id)

	// First check if the animation exists
	if !AnimationExists(id) {
		LogResponse("/animation/{id}", "Animation not found with ID: "+id, nil)
		EncodeError(w, "Animation not found", http.StatusNotFound)
		return
	}

	// Retrieve the animation from the database
	code, description, err := GetAnimation(id)
	if err != nil {
		LogResponse("/animation/{id}", "Error retrieving animation ID: "+id, err)
		// Always keep the Content-Type as application/json for consistent error handling
		EncodeError(w, "Error retrieving animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	LogResponse("/animation/{id}", "Animation retrieved successfully", nil)

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

	LogRequest("/feed", "Retrieving random animation")

	// Retrieve a random animation from the database
	animation, err := GetRandomAnimation()
	if err != nil {
		LogResponse("/feed", "Error retrieving random animation", err)
		EncodeError(w, "Error retrieving random animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	LogResponse("/feed", "Random animation retrieved successfully: "+animation.ID, nil)

	// Return the random animation
	json.NewEncoder(w).Encode(animation)
}

func saveMoodHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req SaveMoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogResponse("/save-mood", "Invalid request format", err)
		EncodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.AnimationID == "" {
		LogResponse("/save-mood", "Animation ID cannot be empty", nil)
		EncodeError(w, "Animation ID cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate mood
	validMood := false
	for _, mood := range []Mood{MoodMuchWorse, MoodWorse, MoodSame, MoodBetter, MoodMuchBetter} {
		if req.Mood == mood {
			validMood = true
			break
		}
	}
	if !validMood {
		LogResponse("/save-mood", "Invalid mood value", nil)
		EncodeError(w, "Invalid mood value", http.StatusBadRequest)
		return
	}

	// Check if animation exists
	if !AnimationExists(req.AnimationID) {
		LogResponse("/save-mood", "Animation not found with ID: "+req.AnimationID, nil)
		EncodeError(w, "Animation not found", http.StatusNotFound)
		return
	}

	// Get user ID from context
	userId, ok := GetUserIDFromContext(r.Context())
	if !ok {
		LogResponse("/save-mood", "User ID missing from context", nil)
		EncodeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Save the mood to the database
	err := SaveMood(userId, req.AnimationID, string(req.Mood))
	if err != nil {
		LogResponse("/save-mood", "Error saving mood", err)
		EncodeError(w, "Error saving mood: "+err.Error(), http.StatusInternalServerError)
		return
	}

	LogResponse("/save-mood", "Mood saved successfully", nil)

	// Return success response
	response := SaveMoodResponse{Success: true}
	json.NewEncoder(w).Encode(response)
}
