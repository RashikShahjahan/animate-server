package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// setupRouter configures and returns the application router
func setupRouter() *mux.Router {
	r := mux.NewRouter()

	// Add middlewares
	r.Use(corsMiddleware)
	r.Use(loggingMiddleware)

	// Set up routes
	r.HandleFunc("/generate-animation", animationHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/save-animation", saveAnimationHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/fix-animation", fixAnimationHandler).Methods(http.MethodPost, http.MethodOptions)

	// You could also add a route to retrieve an animation by ID
	r.HandleFunc("/animation/{id}", getAnimationHandler).Methods(http.MethodGet)

	return r
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

	// Retrieve the animation from the database
	code, description, err := getAnimation(id)
	if err != nil {
		logResponse("/animation/{id}", "Error retrieving animation ID: "+id, err)
		encodeError(w, "Error retrieving animation: "+err.Error(), http.StatusNotFound)
		return
	}

	logResponse("/animation/{id}", "Animation retrieved successfully", nil)

	// Return the animation code
	response := GetAnimationResponse{Code: code, Description: description}
	json.NewEncoder(w).Encode(response)
}

func fixAnimationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request body
	var req FixAnimationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logResponse("/fix-animation", "Invalid request format", err)
		encodeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.BrokenCode == "" {
		logResponse("/fix-animation", "Broken code cannot be empty", nil)
		encodeError(w, "Broken code cannot be empty", http.StatusBadRequest)
		return
	}

	if req.ErrorMessage == "" {
		logResponse("/fix-animation", "Error message cannot be empty", nil)
		encodeError(w, "Error message cannot be empty", http.StatusBadRequest)
		return
	}

	logRequest("/fix-animation", "Error message: "+req.ErrorMessage)

	// Get Claude API key from environment variable
	claudeAPIKey := getAPIKey("CLAUDE_API_KEY")
	if claudeAPIKey == "" {
		logResponse("/fix-animation", "Claude API key not configured", nil)
		encodeError(w, "Claude API key not configured", http.StatusInternalServerError)
		return
	}

	// Fix animation with Claude
	fixedCode, err := fixAnimationWithClaude(req.BrokenCode, req.ErrorMessage, claudeAPIKey)
	if err != nil {
		logResponse("/fix-animation", "Error fixing animation", err)
		encodeError(w, "Error fixing animation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sanitize the animation code by removing markdown fences
	fixedCode = sanitizeSketchCode(fixedCode)

	logResponse("/fix-animation", "Animation fixed successfully", nil)

	// Return the fixed animation code
	response := AnimationResponse{Code: fixedCode}
	json.NewEncoder(w).Encode(response)
}
