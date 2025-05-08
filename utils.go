package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Context utilities for user authentication

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// User context key
const userIDKey contextKey = "userID"

// setUserIDInContext adds a user ID to the request context
func setUserIDInContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// getUserIDFromContext retrieves the user ID from the request context
func getUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		return "", errors.New("user ID not found in context")
	}
	return userID, nil
}

// logRequest logs the request details
func logRequest(endpoint, message string) {
	log.Printf("[REQUEST] %s - %s", endpoint, message)
}

// logResponse logs the response details
func logResponse(endpoint, message string, err error) {
	if err != nil {
		log.Printf("[RESPONSE] %s - %s: %v", endpoint, message, err)
	} else {
		log.Printf("[RESPONSE] %s - %s", endpoint, message)
	}
}

// encodeError encodes an error message as JSON and sends it with the provided status code
func encodeError(w http.ResponseWriter, message string, statusCode int) {
	response := AnimationResponse{Error: message}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// getAPIKey retrieves an API key from environment variables
func getAPIKey(keyName string) string {
	// Load environment variables if needed
	if os.Getenv(keyName) == "" {
		if err := loadEnvFile(); err != nil {
			log.Printf("Warning: Failed to load environment variables: %v", err)
		}
	}

	// Get the API key
	apiKey := os.Getenv(keyName)
	if apiKey == "" {
		log.Printf("Warning: API key '%s' not found in environment variables", keyName)
	}

	return apiKey
}

// loadEnvFile loads environment variables from .env file
func loadEnvFile() error {
	// Open .env file
	envFile, err := os.Open(".env")
	if err != nil {
		if os.IsNotExist(err) {
			// Try env.example instead
			envFile, err = os.Open("env.example")
			if err != nil {
				return fmt.Errorf("no .env or env.example file found: %v", err)
			}
		} else {
			return fmt.Errorf("failed to open .env file: %v", err)
		}
	}
	defer envFile.Close()

	// Read .env file
	content, err := io.ReadAll(envFile)
	if err != nil {
		return fmt.Errorf("failed to read .env file: %v", err)
	}

	// Parse .env file
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		// Set environment variable
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return nil
}

// generateAnimationWithClaude uses Claude API to generate animation based on the description
func generateAnimationWithClaude(description, apiKey string) (string, error) {
	// Construct the prompt for Claude
	prompt := `You're a p5.js expert. Create a beautiful and impressive p5.js animation based on this description: "` + description + `".

Please ONLY provide the complete, working p5.js code with no explanations or markdown. 
The code should be clean, well-commented, and immediately ready to run in a browser.
The animation should be visually impressive and fully utilize p5.js capabilities.
The animation should work in a standard p5.js setup with a 800x500 canvas.
Include all necessary setup() and draw() functions.
Don't include any HTML boilerplate, just the complete p5.js JavaScript code.
`

	// Create Claude API request
	claudeRequest := ClaudeRequest{
		Model: "claude-3-opus-20240229",
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.2,
	}

	// Call Claude API
	animation, err := callClaudeAPI(claudeRequest, apiKey)
	if err != nil {
		return "", err
	}

	return animation, nil
}

// fixAnimationWithClaude uses Claude API to fix broken animation code
func fixAnimationWithClaude(brokenCode, errorMessage, apiKey string) (string, error) {
	// Construct the prompt for Claude
	prompt := `You're a p5.js expert. Fix this broken p5.js animation code which is giving the following error: "` + errorMessage + `"

Here's the broken code:
` + "```" + `
` + brokenCode + `
` + "```" + `

Please ONLY provide the complete fixed p5.js code with no explanations or markdown.
The code should be clean, well-commented, and immediately ready to run in a browser.
Don't include any HTML boilerplate, just the complete p5.js JavaScript code.
`

	// Create Claude API request
	claudeRequest := ClaudeRequest{
		Model: "claude-3-opus-20240229",
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.2,
	}

	// Call Claude API
	fixedCode, err := callClaudeAPI(claudeRequest, apiKey)
	if err != nil {
		return "", err
	}

	return fixedCode, nil
}

// callClaudeAPI makes a request to the Claude API
func callClaudeAPI(req ClaudeRequest, apiKey string) (string, error) {
	// Prepare the HTTP request
	claudeURL := "https://api.anthropic.com/v1/messages"
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Claude request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", claudeURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Claude API returned error: %s (status code: %d)", string(body), resp.StatusCode)
	}

	// Parse the response
	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Claude response: %v", err)
	}

	// Extract the text from the response
	var responseText string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			responseText += content.Text
		}
	}

	return responseText, nil
}

// sanitizeSketchCode removes any markdown fences from the code
func sanitizeSketchCode(code string) string {
	// Remove any leading markdown code fences
	code = strings.TrimPrefix(code, "```javascript")
	code = strings.TrimPrefix(code, "```js")
	code = strings.TrimPrefix(code, "```")

	// Remove any trailing markdown code fences
	code = strings.TrimSuffix(code, "```")

	// Trim any whitespace
	code = strings.TrimSpace(code)

	return code
}
