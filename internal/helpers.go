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

// GenerateAnimationWithClaude calls Claude API to generate p5.js animation from description
func GenerateAnimationWithClaude(description string, apiKey string) (string, error) {
	log.Printf("[CLAUDE] Generating animation for description: %s", description)

	// Prepare the Claude API request
	prompt := `Create a p5.js animation based on this description: "` + description + `". ` +
		`Your response should ONLY include valid JavaScript code that creates a p5.js sketch. The code should:
1. Use p5.js functions like setup() and draw()
2. Create a canvas that fits the container with id "animation-container"
3. Include proper animation logic in the draw() function
4. Be self-contained and ready to run with p5.js library

Example structure:
// p5.js sketch setup
function setup() {
    let canvas = createCanvas(windowWidth, windowHeight);
    canvas.parent('animation-container');
    // Initialize your variables here
}

function draw() {
    // Clear background
    background(220);
    
    // Your animation logic here
    // Use frameCount for time-based animations
}

// Handle window resize
function windowResized() {
    resizeCanvas(windowWidth, windowHeight);
}

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
	codeBlockRegex := regexp.MustCompile("(?s)```(?:javascript|js)?\n?(.*?)\n?```")
	if matches := codeBlockRegex.FindStringSubmatch(raw); len(matches) > 1 {
		raw = matches[1]
	}

	// Remove any leading/trailing whitespace
	raw = strings.TrimSpace(raw)

	return raw
}

// PreprocessP5Code applies comprehensive preprocessing to p5.js code
func PreprocessP5Code(code string) string {
	lines := strings.Split(code, "\n")
	processedLines := make([]string, 0, len(lines))
	declaredVars := make(map[string]bool)

	// First pass: collect already declared variables and function names
	for _, line := range lines {
		// Look for let/var/const declarations
		letRegex := regexp.MustCompile(`(?:let|var|const)\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`)
		if matches := letRegex.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 {
					declaredVars[match[1]] = true
				}
			}
		}

		// Look for function declarations
		funcRegex := regexp.MustCompile(`function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`)
		if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
			declaredVars[matches[1]] = true
		}

		// Look for array declarations like: let arrayName = [];
		arrayRegex := regexp.MustCompile(`(?:let|var|const)\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*\[`)
		if matches := arrayRegex.FindStringSubmatch(line); len(matches) > 1 {
			declaredVars[matches[1]] = true
		}
	}

	// Second pass: fix undeclared variables and other issues
	for _, line := range lines {
		processedLine := line

		// Remove canvas variable assignment, preserve original parameters
		canvasRegex := regexp.MustCompile(`(\s*)(?:let|var|const)\s+canvas\s*=\s*createCanvas\(([^)]*)\);`)
		if matches := canvasRegex.FindStringSubmatch(line); len(matches) > 2 {
			processedLine = matches[1] + "createCanvas(" + matches[2] + ");"
		}

		// Remove or comment out canvas.parent() calls
		parentRegex := regexp.MustCompile(`(\s*).*\.parent\([^)]*\);?\s*`)
		if parentRegex.MatchString(line) {
			processedLine = parentRegex.ReplaceAllString(line, "${1}// Canvas parent handled by instance mode\n")
		}

		// Fix missing closing brackets in array access
		bracketRegex := regexp.MustCompile(`(\w+)\[(\w+)\.(\w+)\s*(\+|-|\*|\/|)=\s*([^;]+);`)
		processedLine = bracketRegex.ReplaceAllString(processedLine, "$1[$2].$3 $4= $5;")

		// Fix undeclared variables
		assignmentRegex := regexp.MustCompile(`^\s*([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*[^=]`)
		if matches := assignmentRegex.FindStringSubmatch(line); len(matches) > 1 {
			varName := matches[1]
			p5Functions := map[string]bool{
				"setup": true, "draw": true, "mousePressed": true, "mouseReleased": true,
				"keyPressed": true, "keyReleased": true, "windowResized": true,
			}

			// Get only the code part before any comment
			codePart := strings.Split(line, "//")[0]

			if !strings.Contains(codePart, "function") &&
				!strings.Contains(codePart, "let ") &&
				!strings.Contains(codePart, "var ") &&
				!strings.Contains(codePart, "const ") &&
				!strings.Contains(codePart, "for ") && // Don't fix for loop variables
				!strings.Contains(codePart, "if ") && // Don't fix if statement assignments
				!declaredVars[varName] &&
				!p5Functions[varName] {

				whitespaceRegex := regexp.MustCompile(`^(\s*)([a-zA-Z_$][a-zA-Z0-9_$]*\s*=)`)
				processedLine = whitespaceRegex.ReplaceAllString(processedLine, "${1}let $2")
				declaredVars[varName] = true
			}
		}

		processedLines = append(processedLines, processedLine)
	}

	return strings.Join(processedLines, "\n")
}

// AnalyzeP5Code analyzes p5.js code and returns metadata about functions found
func AnalyzeP5Code(code string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Detect p5.js functions
	functions := make(map[string]bool)
	functionRegex := regexp.MustCompile(`function\s+(setup|draw|mousePressed|mouseReleased|keyPressed|keyReleased|windowResized)\s*\(`)

	matches := functionRegex.FindAllStringSubmatch(code, -1)
	for _, match := range matches {
		if len(match) > 1 {
			functions[match[1]] = true
		}
	}

	metadata["functions"] = functions
	metadata["hasSetup"] = functions["setup"]
	metadata["hasDraw"] = functions["draw"]
	metadata["hasInteraction"] = functions["mousePressed"] || functions["mouseReleased"] || functions["keyPressed"] || functions["keyReleased"]

	// Detect canvas creation
	canvasRegex := regexp.MustCompile(`createCanvas\s*\(\s*([^,)]+)(?:\s*,\s*([^)]+))?\s*\)`)
	if matches := canvasRegex.FindStringSubmatch(code); len(matches) > 1 {
		metadata["hasCanvas"] = true
		metadata["canvasWidth"] = strings.TrimSpace(matches[1])
		if len(matches) > 2 && matches[2] != "" {
			metadata["canvasHeight"] = strings.TrimSpace(matches[2])
		}
	} else {
		metadata["hasCanvas"] = false
	}

	// Basic validation
	errors := make([]string, 0)
	if !functions["setup"] {
		errors = append(errors, "Missing setup() function")
	}
	if !functions["draw"] {
		errors = append(errors, "Missing draw() function")
	}

	metadata["errors"] = errors
	metadata["isValid"] = len(errors) == 0

	return metadata
}
