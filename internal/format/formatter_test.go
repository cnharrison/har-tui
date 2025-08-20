package format

import (
	"strings"
	"testing"
)

func TestContentFormatter_FormatContent(t *testing.T) {
	formatter := NewContentFormatter()

	tests := []struct {
		name        string
		content     string
		contentType string
		shouldColor bool // Check if output contains color codes
	}{
		{
			name:        "empty content",
			content:     "",
			contentType: "json",
			shouldColor: false, // Should return dim message
		},
		{
			name:        "json content",
			content:     `{"key": "value", "number": 42}`,
			contentType: "json",
			shouldColor: true,
		},
		{
			name:        "javascript content with template literals",
			content:     "const msg = `Hello ${name}!`; console.log(msg);",
			contentType: "javascript",
			shouldColor: true,
		},
		{
			name:        "css content",
			content:     ".class { color: red; background: blue; }",
			contentType: "css",
			shouldColor: true,
		},
		{
			name:        "html content",
			content:     `<div class="test">Hello World</div>`,
			contentType: "html",
			shouldColor: true,
		},
		{
			name:        "xml content",
			content:     `<?xml version="1.0"?><root><item>test</item></root>`,
			contentType: "xml",
			shouldColor: true,
		},
		{
			name:        "binary content hex preview",
			content:     "RIFF\xff\xffWEBPVP8X\x00\x00\x00\x00",
			contentType: "binary",
			shouldColor: true,
		},
		{
			name:        "unknown content type",
			content:     "plain text content",
			contentType: "unknown",
			shouldColor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatContent(tt.content, tt.contentType)

			// Check that we got some output
			if result == "" {
				t.Error("Expected non-empty result")
			}

			if tt.shouldColor {
				// Check if result contains tview color codes
				hasColorCodes := strings.Contains(result, "[") && strings.Contains(result, "]")
				if !hasColorCodes {
					t.Errorf("Expected colored output, but got: %s", result)
				}
			}

			// For empty content, should return the dim message
			if tt.content == "" && !strings.Contains(result, "No content") {
				t.Errorf("Expected 'No content' message for empty input, got: %s", result)
			}
		})
	}
}

func TestContentFormatter_DetectContentType(t *testing.T) {
	formatter := NewContentFormatter()

	tests := []struct {
		name        string
		content     string
		mimeType    string
		expected    string
	}{
		{
			name:     "detect json from mime",
			content:  `{"key": "value"}`,
			mimeType: "application/json",
			expected: "json",
		},
		{
			name:     "detect javascript from mime",
			content:  "console.log('test');",
			mimeType: "application/javascript",
			expected: "javascript",
		},
		{
			name:     "detect html from content",
			content:  "<!DOCTYPE html><html><body>Test</body></html>",
			mimeType: "",
			expected: "html",
		},
		{
			name:     "detect json from content structure",
			content:  `{"api": "response", "data": [1, 2, 3]}`,
			mimeType: "",
			expected: "json",
		},
		{
			name:     "detect javascript from keywords",
			content:  "function test() { var x = true; return x; }",
			mimeType: "",
			expected: "javascript",
		},
		{
			name:     "detect css from mime",
			content:  "body { margin: 0; } .class { background: #fff; }",
			mimeType: "text/css",
			expected: "css",
		},
		{
			name:     "detect xml from declaration",
			content:  `<?xml version="1.0" encoding="UTF-8"?><root></root>`,
			mimeType: "",
			expected: "xml",
		},
		{
			name:     "detect image from webp mime",
			content:  "RIFF\xff\xffWEBPVP8X\x00\x00\x00\x00",
			mimeType: "image/webp",
			expected: "image",
		},
		{
			name:     "detect image from png with octet-stream mime", 
			content:  "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR",
			mimeType: "application/octet-stream",
			expected: "image",
		},
		{
			name:     "detect binary from generic binary data",
			content:  "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f",
			mimeType: "application/octet-stream",
			expected: "binary",
		},
		{
			name:     "detect svg from mime type",
			content:  `<svg xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="40"/></svg>`,
			mimeType: "image/svg+xml",
			expected: "svg",
		},
		{
			name:     "detect svg from content",
			content:  `<svg xmlns="http://www.w3.org/2000/svg"><rect width="100" height="100"/></svg>`,
			mimeType: "",
			expected: "svg",
		},
		{
			name:     "fallback to text",
			content:  "This is just plain text content",
			mimeType: "",
			expected: "text",
		},
		{
			name:     "empty content",
			content:  "",
			mimeType: "",
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.DetectContentType(tt.content, tt.mimeType)
			if result != tt.expected {
				t.Errorf("DetectContentType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContentFormatter_ConvertANSIToTview(t *testing.T) {
	formatter := NewContentFormatter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "convert 256-color cyan",
			input:    "\x1b[38;5;81mcyan text\x1b[0m",
			expected: "[cyan]cyan text[-]",
		},
		{
			name:     "convert multiple 256-colors",
			input:    "\x1b[38;5;148mgreen\x1b[0m and \x1b[38;5;186myellow\x1b[0m",
			expected: "[green]green[-] and [yellow]yellow[-]",
		},
		{
			name:     "remove unknown ANSI codes",
			input:    "\x1b[48;5;196mbackground\x1b[0m",
			expected: "background[-]",
		},
		{
			name:     "plain text unchanged",
			input:    "no colors here",
			expected: "no colors here",
		},
		{
			name:     "common monokai colors",
			input:    "\x1b[38;5;197moperator\x1b[0m \x1b[38;5;231mtext\x1b[0m",
			expected: "[red]operator[-] [white]text[-]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.convertANSIToTview(tt.input)
			if result != tt.expected {
				t.Errorf("convertANSIToTview() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestContentFormatter_PrettyJSON(t *testing.T) {
	formatter := NewContentFormatter()

	tests := []struct {
		name     string
		input    interface{}
		wantType string // We can't predict exact output due to colors, but we can check type
	}{
		{
			name:     "nil input",
			input:    nil,
			wantType: "null",
		},
		{
			name:     "simple object",
			input:    map[string]interface{}{"key": "value"},
			wantType: "json", // Should contain JSON structure
		},
		{
			name:     "array input",
			input:    []int{1, 2, 3},
			wantType: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.PrettyJSON(tt.input)

			switch tt.wantType {
			case "null":
				if !strings.Contains(result, "null") {
					t.Errorf("Expected null output, got: %s", result)
				}
			case "json":
				// Should contain some JSON structure indicators
				hasJsonStructure := strings.Contains(result, "{") || strings.Contains(result, "[")
				if !hasJsonStructure {
					t.Errorf("Expected JSON structure, got: %s", result)
				}
			}
		})
	}
}

func BenchmarkContentFormatter_FormatContent(b *testing.B) {
	formatter := NewContentFormatter()
	jsContent := `
		function fibonacci(n) {
			if (n <= 1) return n;
			return fibonacci(n - 1) + fibonacci(n - 2);
		}
		
		const result = fibonacci(10);
		console.log("Result: " + result);
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatter.FormatContent(jsContent, "javascript")
	}
}