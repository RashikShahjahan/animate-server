package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Context utilities for user authentication

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// User context key
const userIDKey contextKey = "userID"

// SetUserIDInContext adds a user ID to the request context
func SetUserIDInContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserIDFromContext retrieves the user ID from the request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

// LogRequest logs the request details
func LogRequest(endpoint, message string) {
	log.Printf("[REQUEST] %s - %s", endpoint, message)
}

// LogResponse logs the response details
func LogResponse(endpoint, message string, err error) {
	if err != nil {
		log.Printf("[RESPONSE] %s - %s: %v", endpoint, message, err)
	} else {
		log.Printf("[RESPONSE] %s - %s", endpoint, message)
	}
}

// GetAPIKey retrieves an API key from environment variables
func GetAPIKey(keyName string) string {
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

// GenerateAnimationWithClaude calls Claude API to generate Three.js animation from description
func GenerateAnimationWithClaude(description string, apiKey string) (string, error) {
	log.Printf("[CLAUDE] Generating animation for description: %s", description)

	// Prepare the Claude API request
	prompt := `Create a Three.js animation based on this description: "` + description + `". ` +
		`Your response should ONLY include valid JavaScript code that creates a Three.js scene. The code should:
1. Create a scene, camera, and renderer
2. Add the renderer to a DOM element with id "animation-container"
3. Include an animation loop using requestAnimationFrame
4. Be self-contained and ready to run

Example structure:
// Create scene, camera, and renderer
const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
const renderer = new THREE.WebGLRenderer();
renderer.setSize(window.innerWidth, window.innerHeight);
document.getElementById('animation-container').appendChild(renderer.domElement);

// Add your objects and lights here
// ...

// Animation loop
function animate() {
    requestAnimationFrame(animate);
    // Animation logic here
    renderer.render(scene, camera);
}
animate();

Do not include any markdown, HTML, CSS, or explanations. Only return the JavaScript code.`

	claudeReq := ClaudeRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   8192,
		Temperature: 1.0,
	}

	// Convert request to JSON
	reqBody, err := json.Marshal(claudeReq)
	if err != nil {
		log.Printf("[CLAUDE ERROR] Failed to marshal request: %v", err)
		return "", err
	}

	// Create HTTP request to Claude API
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("[CLAUDE ERROR] Failed to create request: %v", err)
		return "", err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Send the request
	log.Printf("[CLAUDE] Sending request to API")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CLAUDE ERROR] Failed to send request: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[CLAUDE ERROR] Failed to read response: %v", err)
		return "", err
	}

	// Parse the response
	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		log.Printf("[CLAUDE ERROR] Failed to unmarshal response: %v", err)
		return "", err
	}

	log.Printf("[CLAUDE] Response received successfully")

	// Extract the animation code from the response
	var animationCode string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			animationCode += content.Text
		}
	}

	return animationCode, nil
}

// EncodeError writes a JSON error response
func EncodeError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	response := struct {
		Error  string `json:"error"`
		Status int    `json:"status"`
	}{
		Error:  message,
		Status: statusCode,
	}
	json.NewEncoder(w).Encode(response)
}

// SanitizeAnimationCode cleans up the raw JavaScript code from Claude
func SanitizeAnimationCode(raw string) string {
	// Remove markdown code blocks if present
	codeBlockRegex := regexp.MustCompile("```(?:javascript|js)?\n?(.*?)\n?```")
	if matches := codeBlockRegex.FindStringSubmatch(raw); len(matches) > 1 {
		raw = matches[1]
	}

	// Remove any leading/trailing whitespace
	raw = strings.TrimSpace(raw)

	return raw
}
