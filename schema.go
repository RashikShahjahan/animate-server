package main

// AnimationRequest represents the request for animation generation
type AnimationRequest struct {
	Description string `json:"description"`
}

// AnimationResponse represents the response with p5.js animation
type AnimationResponse struct {
	Code  string `json:"code"`
	Error string `json:"error,omitempty"`
}

type SaveAnimationRequest struct {
	Code string `json:"code"`
}

type SaveAnimationResponse struct {
	ID string `json:"id"`
}

type GetAnimationRequest struct {
	ID string `json:"id"`
}

type GetAnimationResponse struct {
	Code string `json:"code"`
}

type FixAnimationRequest struct {
	BrokenCode   string `json:"broken_code"`
	ErrorMessage string `json:"error_message"`
}

// Claude API request structure
type ClaudeRequest struct {
	Model       string          `json:"model"`
	Messages    []ClaudeMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

// ClaudeMessage represents a message in the Claude conversation
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Claude API response structure
type ClaudeResponse struct {
	Content []ClaudeContent `json:"content"`
}

// ClaudeContent represents content in Claude's response
type ClaudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
