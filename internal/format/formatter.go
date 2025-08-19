package format

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-xmlfmt/xmlfmt"
	"github.com/yosssi/gohtml"
)

// ContentFormatter handles formatting of various content types
type ContentFormatter struct{}

// NewContentFormatter creates a new content formatter
func NewContentFormatter() *ContentFormatter {
	return &ContentFormatter{}
}

// FormatContent formats content based on detected type with syntax highlighting
func (f *ContentFormatter) FormatContent(content, contentType string) string {
	if content == "" {
		return "[dim]No content[white]"
	}

	switch contentType {
	case "json":
		return f.formatJSON(content)
	case "html":
		return f.formatHTML(content)
	case "xml":
		return f.formatXML(content)
	case "javascript":
		return f.formatJavaScript(content)
	case "css":
		return f.formatCSS(content)
	default:
		return content
	}
}

// DetectContentType detects content type from content and MIME type
func (f *ContentFormatter) DetectContentType(content, mimeType string) string {
	// First check explicit MIME type
	if mimeType != "" {
		lowerMime := strings.ToLower(mimeType)
		switch {
		case strings.Contains(lowerMime, "json"):
			return "json"
		case strings.Contains(lowerMime, "html"):
			return "html"
		case strings.Contains(lowerMime, "javascript") || strings.Contains(lowerMime, "/js") || strings.Contains(lowerMime, "ecmascript"):
			return "javascript"
		case strings.Contains(lowerMime, "css"):
			return "css"
		case strings.Contains(lowerMime, "xml"):
			return "xml"
		case strings.Contains(lowerMime, "text/plain"):
			// Fall through to content-based detection for text/plain
			break
		}
	}
	
	// Use Go's built-in content detection
	if content != "" {
		detectedType := http.DetectContentType([]byte(content))
		switch {
		case strings.Contains(detectedType, "text/html"):
			return "html"
		case strings.Contains(detectedType, "text/xml") || strings.Contains(detectedType, "application/xml"):
			return "xml"
		case strings.Contains(detectedType, "application/json"):
			return "json"
		}
	}
	
	// Enhanced content-based detection
	trimmed := strings.TrimSpace(content)
	if len(trimmed) == 0 {
		return "text"
	}
	
	// JSON detection with validation
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
	   (strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		var jsonData interface{}
		if json.Unmarshal([]byte(trimmed), &jsonData) == nil {
			return "json"
		}
	}
	
	// XML detection with validation  
	if strings.HasPrefix(trimmed, "<?xml") {
		return "xml"
	}
	if regexp.MustCompile(`^<[a-zA-Z][^>]*>.*</[a-zA-Z][^>]*>$`).MatchString(strings.ReplaceAll(trimmed, "\n", "")) {
		// Try to parse as XML to verify
		var xmlData interface{}
		if xml.Unmarshal([]byte(trimmed), &xmlData) == nil {
			return "xml"
		}
	}
	
	// HTML detection
	lowerTrimmed := strings.ToLower(trimmed)
	if strings.Contains(lowerTrimmed, "<!doctype html") ||
	   strings.Contains(lowerTrimmed, "<html") ||
	   regexp.MustCompile(`<(div|span|p|body|head|script|style|link|meta)\b[^>]*>`).MatchString(lowerTrimmed) {
		return "html"
	}
	
	// JavaScript detection (enhanced patterns)
	jsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(function|var|let|const|class|import|export)\b`),
		regexp.MustCompile(`\b(console|window|document)\.[a-zA-Z]`),
		regexp.MustCompile(`=>\s*[{(]`),
		regexp.MustCompile(`\b(true|false|null|undefined)\b`),
		regexp.MustCompile(`//.*$`),
		regexp.MustCompile(`/\*.*\*/`),
		regexp.MustCompile(`\.(getElementById|addEventListener|querySelector)`),
	}
	for _, pattern := range jsPatterns {
		if pattern.MatchString(trimmed) {
			return "javascript"
		}
	}
	
	// CSS detection (enhanced)
	cssPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[.#]?[a-zA-Z][\w-]*\s*\{[^}]*\}`),
		regexp.MustCompile(`@(media|import|keyframes|font-face)\b`),
		regexp.MustCompile(`\b(color|background|margin|padding|font-size|width|height)\s*:`),
	}
	for _, pattern := range cssPatterns {
		if pattern.MatchString(trimmed) {
			return "css"
		}
	}
	
	return "text"
}

// formatJSON formats and highlights JSON content
func (f *ContentFormatter) formatJSON(content string) string {
	var jsonData interface{}
	if json.Unmarshal([]byte(content), &jsonData) == nil {
		formatted, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return content
		}
		
		result := string(formatted)
		// Enhanced JSON syntax highlighting
		result = regexp.MustCompile(`([\[\]{}])`).ReplaceAllString(result, `[blue]$1[white]`)
		result = regexp.MustCompile(`"([^"]+)"(\s*:)`).ReplaceAllString(result, `[cyan]"$1"[white]$2`)
		result = regexp.MustCompile(`:\s*"([^"]*)"`).ReplaceAllString(result, `: [green]"$1"[white]`)
		result = regexp.MustCompile(`:\s*(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)`).ReplaceAllString(result, `: [yellow]$1[white]`)
		result = regexp.MustCompile(`:\s*(true|false|null)`).ReplaceAllString(result, `: [magenta]$1[white]`)
		return result
	}
	return content
}

// formatHTML formats and highlights HTML content
func (f *ContentFormatter) formatHTML(content string) string {
	formatted := gohtml.Format(content)
	formatted = regexp.MustCompile(`(<[^/>]+>)`).ReplaceAllString(formatted, `[blue]$1[white]`)
	formatted = regexp.MustCompile(`(</[^>]+>)`).ReplaceAllString(formatted, `[blue]$1[white]`)
	formatted = regexp.MustCompile(`\s(\w+)="`).ReplaceAllString(formatted, ` [cyan]$1[white]="`)
	formatted = regexp.MustCompile(`="([^"]*)"`).ReplaceAllString(formatted, `="[green]$1[white]"`)
	formatted = regexp.MustCompile(`(<!--.*?-->)`).ReplaceAllString(formatted, `[dim]$1[white]`)
	return formatted
}

// formatXML formats and highlights XML content
func (f *ContentFormatter) formatXML(content string) string {
	formatted := xmlfmt.FormatXML(content, "", "  ")
	formatted = regexp.MustCompile(`(<\?[^>]*\?>)`).ReplaceAllString(formatted, `[magenta]$1[white]`)
	formatted = regexp.MustCompile(`(<[^/>]+>)`).ReplaceAllString(formatted, `[blue]$1[white]`)
	formatted = regexp.MustCompile(`(</[^>]+>)`).ReplaceAllString(formatted, `[blue]$1[white]`)
	formatted = regexp.MustCompile(`\s(\w+)="`).ReplaceAllString(formatted, ` [cyan]$1[white]="`)
	formatted = regexp.MustCompile(`="([^"]*)"`).ReplaceAllString(formatted, `="[green]$1[white]"`)
	formatted = regexp.MustCompile(`(<!--.*?-->)`).ReplaceAllString(formatted, `[dim]$1[white]`)
	return formatted
}

// formatJavaScript formats and highlights JavaScript content
func (f *ContentFormatter) formatJavaScript(content string) string {
	formatted := f.formatJSIndentation(content)
	
	// Keywords with word boundaries
	keywords := []string{
		"function", "var", "let", "const", "if", "else", "for", "while", "do",
		"return", "true", "false", "null", "undefined", "new", "this", "class",
		"extends", "import", "export", "from", "default", "async", "await",
		"try", "catch", "finally", "throw", "typeof", "instanceof", "in", "of",
		"break", "continue", "switch", "case", "with", "yield",
	}
	for _, keyword := range keywords {
		pattern := `\b` + regexp.QuoteMeta(keyword) + `\b`
		formatted = regexp.MustCompile(pattern).ReplaceAllString(formatted, `[magenta]`+keyword+`[white]`)
	}
	
	// Comments
	formatted = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(formatted, `[dim]$0[white]`)
	formatted = regexp.MustCompile(`//.*?(?:\n|$)`).ReplaceAllString(formatted, `[dim]$0[white]`)
	
	// String literals
	formatted = regexp.MustCompile("`` ([^`` \\\\]|\\\\.|`` )*`` ").ReplaceAllString(formatted, `[green]$0[white]`)
	formatted = regexp.MustCompile(`"([^"\\\\]|\\\\.)*"`).ReplaceAllString(formatted, `[green]$0[white]`)
	formatted = regexp.MustCompile(`'([^'\\\\]|\\\\.)*'`).ReplaceAllString(formatted, `[green]$0[white]`)
	
	// Numbers
	formatted = regexp.MustCompile(`\b\d+(\.\d+)?([eE][+-]?\d+)?\b`).ReplaceAllString(formatted, `[yellow]$0[white]`)
	
	// Object properties and method calls
	formatted = regexp.MustCompile(`\.([a-zA-Z_$][a-zA-Z0-9_$]*)`).ReplaceAllString(formatted, `.[cyan]$1[white]`)
	formatted = regexp.MustCompile(`([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`).ReplaceAllString(formatted, `[blue]$1[white](`)
	
	// Operators
	formatted = regexp.MustCompile(`(===?|!==?|[<>]=?|\+=?|-=?|\*=?|/=?|%=?|&=?|\|=?|\^=?|<<=?|>>=?)`).ReplaceAllString(formatted, `[red]$1[white]`)
	
	return formatted
}

// formatJSIndentation provides basic JavaScript indentation
func (f *ContentFormatter) formatJSIndentation(code string) string {
	lines := strings.Split(code, "\n")
	var formatted []string
	indentLevel := 0
	
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			formatted = append(formatted, "")
			continue
		}
		
		// Decrease indent for closing braces
		if strings.HasPrefix(trimmedLine, "}") || strings.HasPrefix(trimmedLine, "]") || strings.HasPrefix(trimmedLine, ")") {
			if indentLevel > 0 {
				indentLevel--
			}
		}
		
		// Add indentation
		indentedLine := strings.Repeat("  ", indentLevel) + trimmedLine
		formatted = append(formatted, indentedLine)
		
		// Increase indent for opening braces
		if strings.HasSuffix(trimmedLine, "{") || strings.HasSuffix(trimmedLine, "[") || strings.HasSuffix(trimmedLine, "(") {
			indentLevel++
		}
	}
	
	return strings.Join(formatted, "\n")
}

// formatCSS formats and highlights CSS content
func (f *ContentFormatter) formatCSS(content string) string {
	formatted := content
	
	// CSS at-rules
	formatted = regexp.MustCompile(`(@[a-zA-Z-]+)`).ReplaceAllString(formatted, `[magenta]$1[white]`)
	
	// Selectors
	formatted = regexp.MustCompile(`([.#]?[a-zA-Z][\w-]*(?::[a-zA-Z-]+)?(?:::[a-zA-Z-]+)?)\s*\{`).ReplaceAllString(formatted, `[cyan]$1[white] {`)
	
	// Properties
	formatted = regexp.MustCompile(`\s*([a-zA-Z-]+)\s*:`).ReplaceAllString(formatted, ` [yellow]$1[white]:`)
	
	// Values
	formatted = regexp.MustCompile(`:\s*([^;}\n]+)`).ReplaceAllString(formatted, `: [green]$1[white]`)
	
	// CSS comments
	formatted = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(formatted, `[dim]$0[white]`)
	
	// Important declarations
	formatted = regexp.MustCompile(`!important`).ReplaceAllString(formatted, `[red]!important[white]`)
	
	return formatted
}

// PrettyJSON formats any data structure as pretty-printed JSON
func (f *ContentFormatter) PrettyJSON(data interface{}) string {
	if data == nil {
		return "[dim]null[white]"
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("[red]Error formatting JSON: %v[white]", err)
	}
	return string(pretty)
}