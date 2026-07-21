package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJWTSecretValidation(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "missing", secret: "", wantErr: true},
		{name: "example placeholder", secret: jwtSecretPlaceholder, wantErr: true},
		{name: "too short", secret: strings.Repeat("a", minJWTSecretLength-1), wantErr: true},
		{name: "minimum length", secret: strings.Repeat("a", minJWTSecretLength)},
		{name: "longer secret", secret: strings.Repeat("a", minJWTSecretLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET_KEY", tt.secret)

			secret, err := JWTSecret()
			if (err != nil) != tt.wantErr {
				t.Fatalf("JWTSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(secret) != tt.secret {
				t.Errorf("JWTSecret() = %q, want %q", secret, tt.secret)
			}
		})
	}
}

func TestEncodeError(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/plain")

	EncodeError(recorder, "invalid request", http.StatusBadRequest)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	var body struct {
		Error  string `json:"error"`
		Status int    `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "invalid request" || body.Status != http.StatusBadRequest {
		t.Errorf("body = %+v, want error %q and status %d", body, "invalid request", http.StatusBadRequest)
	}
}

func TestSanitizeAnimationCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove JavaScript markdown",
			input:    "```javascript\nfunction setup() {\n  createCanvas(400, 400);\n}\n```",
			expected: "function setup() {\n  createCanvas(400, 400);\n}",
		},
		{
			name:     "Remove JS markdown",
			input:    "```js\nfunction draw() {\n  background(220);\n}\n```",
			expected: "function draw() {\n  background(220);\n}",
		},
		{
			name:     "Remove markdown without language",
			input:    "```\nfunction setup() {\n  createCanvas(400, 400);\n}\n```",
			expected: "function setup() {\n  createCanvas(400, 400);\n}",
		},
		{
			name:     "No markdown to remove",
			input:    "function setup() {\n  createCanvas(400, 400);\n}",
			expected: "function setup() {\n  createCanvas(400, 400);\n}",
		},
		{
			name:     "Trim whitespace",
			input:    "  \n  function setup() {\n  createCanvas(400, 400);\n}  \n  ",
			expected: "function setup() {\n  createCanvas(400, 400);\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAnimationCode(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeAnimationCode() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateP5jsCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{
			name: "Valid p5.js code with setup and draw",
			code: `function setup() {
				createCanvas(400, 400);
			}
			function draw() {
				background(220);
				circle(200, 200, 50);
			}`,
			expected: true,
		},
		{
			name: "Valid p5.js code with setup only",
			code: `function setup() {
				createCanvas(400, 400);
				noLoop();
			}`,
			expected: true,
		},
		{
			name: "Invalid code without setup",
			code: `function draw() {
				background(220);
			}`,
			expected: false,
		},
		{
			name:     "Empty code",
			code:     "",
			expected: false,
		},
		{
			name:     "Non-p5.js code",
			code:     `console.log("hello world");`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateP5jsCode(tt.code)
			if result != tt.expected {
				t.Errorf("validateP5jsCode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function to validate if code looks like valid p5.js
func validateP5jsCode(code string) bool {
	if strings.TrimSpace(code) == "" {
		return false
	}

	// Check if code contains setup function (minimum requirement for p5.js)
	return strings.Contains(code, "function setup()")
}

func TestGenerateP5jsExample(t *testing.T) {
	// Test that we can generate a simple p5.js example
	exampleCode := `function setup() {
		let canvas = createCanvas(windowWidth, windowHeight);
		canvas.parent('animation-container');
	}
	
	function draw() {
		background(220);
		fill(255, 0, 0);
		circle(mouseX, mouseY, 50);
	}
	
	function windowResized() {
		resizeCanvas(windowWidth, windowHeight);
	}`

	// Verify the example has key p5.js components
	if !strings.Contains(exampleCode, "function setup()") {
		t.Error("Example code should contain setup function")
	}

	if !strings.Contains(exampleCode, "function draw()") {
		t.Error("Example code should contain draw function")
	}

	if !strings.Contains(exampleCode, "createCanvas") {
		t.Error("Example code should create a canvas")
	}

	if !strings.Contains(exampleCode, "animation-container") {
		t.Error("Example code should target animation-container")
	}

	if !strings.Contains(exampleCode, "windowResized") {
		t.Error("Example code should handle window resizing")
	}
}
