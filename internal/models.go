package internal

import (
	"time"
)

// AnimationRequest represents the request for animation generation
type AnimationRequest struct {
	Description string `json:"description"`
}

// AnimationResponse represents the response with Three.js animation
type AnimationResponse struct {
	Code  string `json:"code"`
	Error string `json:"error,omitempty"`
}

type SaveAnimationRequest struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type SaveAnimationResponse struct {
	ID string `json:"id"`
}

type GetAnimationRequest struct {
	ID string `json:"id"`
}

type GetAnimationResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

type GetAnimationFeedResponse []GetAnimationResponse

type FixAnimationRequest struct {
	BrokenCode   string `json:"broken_code"`
	ErrorMessage string `json:"error_message"`
}

// RegisterRequest represents the user registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse represents the response after successful registration
type RegisterResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// LoginRequest represents the user login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the response after successful login
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// User represents user information
type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	LastLogin *time.Time `json:"lastLogin,omitempty"`
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

// Mood represents a user's mood after viewing an animation
type Mood string

// Valid mood values
const (
	MoodMuchWorse  Mood = "much worse"
	MoodWorse      Mood = "worse"
	MoodSame       Mood = "same"
	MoodBetter     Mood = "better"
	MoodMuchBetter Mood = "much better"
)

// SaveMoodRequest represents the request to save a user's mood
type SaveMoodRequest struct {
	AnimationID string `json:"animationId"`
	Mood        Mood   `json:"mood"`
}

// SaveMoodResponse represents the response from save-mood endpoint
type SaveMoodResponse struct {
	Success bool `json:"success"`
}
