package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// logRequest logs details about an API request
func logRequest(endpoint string, requestDetails string) {
	log.Printf("[REQUEST] %s - %s", endpoint, requestDetails)
}

// logResponse logs details about an API response
func logResponse(endpoint string, responseDetails string, err error) {
	if err != nil {
		log.Printf("[ERROR] %s - %s - %v", endpoint, responseDetails, err)
	} else {
		log.Printf("[RESPONSE] %s - %s", endpoint, responseDetails)
	}
}

// getAPIKey retrieves the API key from environment variables
// Returns the API key or an empty string if not found
func getAPIKey(key string) string {
	return os.Getenv(key)
}

// generateAnimationWithClaude calls Claude API to generate p5.js animation from description
func generateAnimationWithClaude(description string, apiKey string) (string, error) {
	log.Printf("[CLAUDE] Generating animation for description: %s", description)

	// Prepare the Claude API request
	prompt := `Create a p5.js animation based on this description: "` + description + `". ` +
		`Use p5.js instance mode. Your response should ONLY include valid JavaScript code that initializes a p5 instance, for example:
` +
		`new p5(function(p) {
    p.setup = function() { /* setup code */ };
    p.draw = function() { /* draw code */ };
    // helper functions as p.method = function() { ... };
});` +
		`Do not include any markdown, HTML, CSS, or explanations. Prefix all p5 functions and properties with 'p.'.`

	claudeReq := ClaudeRequest{
		Model: "claude-3-7-sonnet-20250219",
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.7,
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

// encodeError writes a JSON error response
func encodeError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	response := AnimationResponse{Error: message}
	json.NewEncoder(w).Encode(response)
}

// sanitizeSketchCode removes Markdown fences, prefixes p5.js method calls with 'p.', and wraps code in a p5 instance
func sanitizeSketchCode(raw string) string {
	// Extract code inside fences or use raw
	fenceRegex := regexp.MustCompile("(?s)```(?:js|javascript)?\\n([\\s\\S]*?)```")
	var code string
	if matches := fenceRegex.FindStringSubmatch(raw); len(matches) > 1 {
		code = matches[1]
	} else {
		code = raw
	}
	// Trim whitespace
	code = strings.TrimSpace(code)
	// Prefix standalone dot calls with p.
	dotRe := regexp.MustCompile(`(\W)\.(\w+)`)
	for i := 0; i < 2; i++ {
		code = dotRe.ReplaceAllString(code, `$1p.$2`)
	}
	// Wrap in p5 instance if not already
	if !strings.HasPrefix(strings.TrimSpace(code), "new p5") {
		code = "new p5(function(p) {\n" + code + "\n});"
	}
	return code
}

// fixAnimationWithClaude calls Claude API to fix broken p5.js animation code
func fixAnimationWithClaude(brokenCode string, errorMessage string, apiKey string) (string, error) {
	log.Printf("[CLAUDE] Fixing animation with error: %s", errorMessage)

	// Prepare the Claude API request
	prompt := `Fix this broken p5.js animation code. Here's the code:

` + brokenCode + `

The error message is:
` + errorMessage + `

Please provide only the fixed JavaScript code that solves this error. Use p5.js instance mode and make sure all p5 functions and properties are prefixed with 'p.'. Return only valid JavaScript code without any markdown, explanations, or comments about the changes.`

	claudeReq := ClaudeRequest{
		Model: "claude-3-7-sonnet-20250219",
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.3,
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

	// Extract the fixed animation code from the response
	var fixedCode string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			fixedCode += content.Text
		}
	}

	return fixedCode, nil
}
