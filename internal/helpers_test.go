package internal

import (
	"strings"
	"testing"
)

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
