package format

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-xmlfmt/xmlfmt"
	"github.com/yosssi/gohtml"
	"golang.org/x/term"
)

// ContentFormatter handles formatting of various content types
type ContentFormatter struct {
	chromaFormatter chroma.Formatter
	chromaStyle     *chroma.Style
	imageDisplayer  *ImageDisplayer // Lazy initialized
}

// NewContentFormatter creates a new content formatter
func NewContentFormatter() *ContentFormatter {
	// Use terminal formatter with 256 colors for better compatibility
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	
	// Use a dark theme that works well in terminals
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	
	return &ContentFormatter{
		chromaFormatter: formatter,
		chromaStyle:     style,
		imageDisplayer:  nil, // Lazy initialized when first needed
	}
}

// getImageDisplayer returns the image displayer, initializing it lazily if needed
func (f *ContentFormatter) getImageDisplayer() *ImageDisplayer {
	if f.imageDisplayer == nil {
		f.imageDisplayer = NewImageDisplayer()
	}
	return f.imageDisplayer
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
		return f.formatWithChroma(content, "javascript")
	case "css":
		return f.formatWithChroma(content, "css")
	case "image":
		return f.formatImagePreview(content)
	case "svg":
		return f.formatSVGPreview(content)
	case "binary":
		return f.formatHexPreview(content)
	default:
		return content
	}
}

// isBinaryContent checks if content appears to be binary data
func (f *ContentFormatter) isBinaryContent(content, mimeType string) bool {
	// Check MIME type first for known binary types
	if mimeType != "" {
		lowerMime := strings.ToLower(mimeType)
		binaryTypes := []string{
			"image/", "video/", "audio/", "application/pdf", "application/zip",
			"application/octet-stream", "font/", "application/x-", "binary/",
		}
		for _, binType := range binaryTypes {
			if strings.Contains(lowerMime, binType) {
				return true
			}
		}
	}
	
	// Check content for binary indicators
	if len(content) == 0 {
		return false
	}
	
	// Count non-printable characters
	nonPrintable := 0
	for i, b := range []byte(content) {
		if i > 512 { // Check first 512 bytes only
			break
		}
		// Consider null bytes and other control chars (except newline, tab, carriage return)
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonPrintable++
		}
		if b > 126 && b < 160 { // Extended ASCII control characters
			nonPrintable++
		}
	}
	
	// If more than 10% are non-printable, consider it binary
	threshold := len(content) / 10
	if threshold < 10 {
		threshold = 10 // Minimum threshold for small content
	}
	
	return nonPrintable > threshold
}

// isImageContent checks if content appears to be image data
func (f *ContentFormatter) isImageContent(content, mimeType string) bool {
	// Check MIME type first for known image types
	if mimeType != "" {
		lowerMime := strings.ToLower(mimeType)
		imageTypes := []string{
			"image/", "image/png", "image/jpeg", "image/jpg", "image/gif", 
			"image/webp", "image/svg+xml", "image/bmp", "image/tiff", "image/ico",
		}
		for _, imgType := range imageTypes {
			if strings.Contains(lowerMime, imgType) {
				return true
			}
		}
	}
	
	// Check content for image signatures if we have content
	if len(content) < 8 {
		return false
	}
	
	data := []byte(content)
	// Check for common image file signatures
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n": // PNG
		return true
	case len(data) >= 3 && (string(data[:3]) == "\xff\xd8\xff"): // JPEG
		return true
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"): // GIF
		return true
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP": // WebP
		return true
	case len(data) >= 2 && (string(data[:2]) == "BM"): // BMP
		return true
	}
	
	return false
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
		case strings.Contains(lowerMime, "image/svg+xml") || strings.Contains(lowerMime, "svg"):
			return "svg"
		case strings.Contains(lowerMime, "xml"):
			return "xml"
		case strings.Contains(lowerMime, "text/plain"):
			// Fall through to content-based detection for text/plain
			break
		default:
			// Check if it's an image first
			if f.isImageContent(content, mimeType) {
				return "image"
			}
			// Check if it's a binary MIME type
			if f.isBinaryContent(content, mimeType) {
				return "binary"
			}
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
	
	// HTML detection
	lowerTrimmed := strings.ToLower(trimmed)
	
	// SVG detection (check before XML since SVG is XML)
	if strings.Contains(lowerTrimmed, "<svg") || strings.Contains(lowerTrimmed, "xmlns=\"http://www.w3.org/2000/svg\"") {
		return "svg"
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
	
	// Check if content appears to be an image first
	if f.isImageContent(trimmed, mimeType) {
		return "image"
	}
	
	// Check if content appears to be binary
	if f.isBinaryContent(trimmed, mimeType) {
		return "binary"
	}

	return "text"
}

// formatHexPreview creates a formatted hex dump view of binary data
func (f *ContentFormatter) formatHexPreview(content string) string {
	data := []byte(content)
	maxBytes := 512 // Limit preview to first 512 bytes to avoid overwhelming the UI
	
	if len(data) == 0 {
		return "[dim]No binary data[white]"
	}
	
	var result strings.Builder
	
	// Header with file info
	result.WriteString(fmt.Sprintf("[yellow]Binary Data Preview[white] ([cyan]%d[white] bytes", len(data)))
	if len(data) > maxBytes {
		result.WriteString(fmt.Sprintf(", showing first [cyan]%d[white]", maxBytes))
		data = data[:maxBytes]
	}
	result.WriteString(")\n\n")
	
	// Hex dump with ASCII preview (like xxd)
	for i := 0; i < len(data); i += 16 {
		// Offset
		result.WriteString(fmt.Sprintf("[blue]%08x:[white] ", i))
		
		// Hex bytes (in groups of 2)
		for j := 0; j < 16; j++ {
			if i+j < len(data) {
				result.WriteString(fmt.Sprintf("[cyan]%02x[white]", data[i+j]))
			} else {
				result.WriteString("  ") // Padding for incomplete lines
			}
			
			// Add space after every 2 bytes for readability
			if j%2 == 1 {
				result.WriteString(" ")
			}
		}
		
		// ASCII representation
		result.WriteString(" [dim]|[white]")
		for j := 0; j < 16 && i+j < len(data); j++ {
			b := data[i+j]
			if b >= 32 && b <= 126 {
				result.WriteString(fmt.Sprintf("[green]%c[white]", b)) // Printable characters in green
			} else {
				result.WriteString("[dim].[white]") // Non-printable as dots
			}
		}
		result.WriteString("[dim]|[white]\n")
	}
	
	// Footer with additional info if truncated
	if len([]byte(content)) > maxBytes {
		remaining := len([]byte(content)) - maxBytes
		result.WriteString(fmt.Sprintf("\n[dim]... and %d more bytes[white]", remaining))
	}
	
	return result.String()
}

// formatImagePreview displays image and binary preview side by side
func (f *ContentFormatter) formatImagePreview(content string) string {
	// Try to display the image using the image displayer
	imageData := []byte(content)
	
	// Detect MIME type from content if possible
	mimeType := ""
	if len(imageData) >= 8 {
		switch {
		case string(imageData[:8]) == "\x89PNG\r\n\x1a\n":
			mimeType = "image/png"
		case len(imageData) >= 3 && string(imageData[:3]) == "\xff\xd8\xff":
			mimeType = "image/jpeg"
		case len(imageData) >= 6 && (string(imageData[:6]) == "GIF87a" || string(imageData[:6]) == "GIF89a"):
			mimeType = "image/gif"
		case len(imageData) >= 12 && string(imageData[:4]) == "RIFF" && string(imageData[8:12]) == "WEBP":
			mimeType = "image/webp"
		case len(imageData) >= 2 && string(imageData[:2]) == "BM":
			mimeType = "image/bmp"
		}
	}
	
	// Try to display the image
	imageOutput, err := f.getImageDisplayer().DisplayImage(imageData, mimeType)
	if err != nil {
		// If image display fails, show error with hex preview
		imageOutput = fmt.Sprintf("[red]Image Display Failed: %v[white]\n[dim]Terminal image display unavailable[white]", err)
	} else {
		// Extract just the image content without headers from DisplayImage output
		imageOutput = f.extractImageContent(imageOutput)
	}
	
	// Create compact hex preview for side-by-side layout
	hexPreview := f.formatCompactHexPreview(content)
	
	// Create side-by-side layout
	return f.createSideBySideLayout("Image Preview", imageOutput, "Binary Data", hexPreview)
}

// formatSVGPreview displays SVG code and image side by side
func (f *ContentFormatter) formatSVGPreview(content string) string {
	// Format the SVG code with syntax highlighting
	xmlFormatted := f.formatXML(content)
	
	// Attempt to render as image
	imagePreview, err := f.getImageDisplayer().RenderSVGAsImage(content)
	if err != nil {
		imagePreview = fmt.Sprintf("[red]SVG rendering failed: %v[white]\n[dim]Image preview unavailable[white]", err)
	}
	
	// Create side-by-side layout - keep preview on left for consistency with image layout
	return f.createSideBySideLayout("SVG Image Preview", imagePreview, "SVG Code", xmlFormatted)
}

// formatWithChroma uses Chroma for syntax highlighting
func (f *ContentFormatter) formatWithChroma(content, language string) string {
	lexer := lexers.Get(language)
	if lexer == nil {
		// Fallback to analyzing content
		lexer = lexers.Analyse(content)
		if lexer == nil {
			return content
		}
	}
	
	// Ensure lexer is configured
	lexer = chroma.Coalesce(lexer)
	
	// Tokenize the content
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content // Fallback to original content
	}
	
	// Format with color codes
	var buf strings.Builder
	err = f.chromaFormatter.Format(&buf, f.chromaStyle, iterator)
	if err != nil {
		return content // Fallback to original content
	}
	
	result := buf.String()
	
	// Convert ANSI color codes to tview color tags
	result = f.convertANSIToTview(result)
	
	return result
}

// convertANSIToTview converts ANSI color codes to tview color tags
func (f *ContentFormatter) convertANSIToTview(content string) string {
	// Handle 256-color ANSI codes first (more specific pattern)
	color256Pattern := regexp.MustCompile(`\x1b\[38;5;(\d+)m`)
	result := color256Pattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract color number from the match
		matches := color256Pattern.FindStringSubmatch(match)
		if len(matches) > 1 {
			colorNum := matches[1]
			return f.map256ColorToTview(colorNum)
		}
		return ""
	})
	
	// Handle reset codes
	result = strings.ReplaceAll(result, "\x1b[0m", "[-]")
	
	// Remove any remaining ANSI escape sequences
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	result = ansiPattern.ReplaceAllString(result, "")
	
	return result
}

// map256ColorToTview maps 256-color ANSI codes to tview colors
func (f *ContentFormatter) map256ColorToTview(colorNum string) string {
	// Map common 256-color codes to tview colors
	// This is a simplified mapping focusing on the most common colors
	switch colorNum {
	// Standard colors (0-15)
	case "0":  return "[black]"
	case "1":  return "[red]"
	case "2":  return "[green]"
	case "3":  return "[yellow]"
	case "4":  return "[blue]"
	case "5":  return "[magenta]"
	case "6":  return "[cyan]"
	case "7":  return "[white]"
	case "8":  return "[gray]"
	case "9":  return "[red]"
	case "10": return "[green]"
	case "11": return "[yellow]"
	case "12": return "[blue]"
	case "13": return "[magenta]"
	case "14": return "[cyan]"
	case "15": return "[white]"
	
	// Common Monokai theme colors used by Chroma
	case "81":  return "[cyan]"     // Keywords (const, function, etc)
	case "148": return "[green]"    // Variables, identifiers
	case "186": return "[yellow]"   // Strings
	case "197": return "[red]"      // Operators
	case "231": return "[white]"    // Default text
	case "244": return "[dim]"      // Comments
	case "208": return "[yellow]"   // Numbers
	case "141": return "[magenta]"  // Special keywords
	case "107": return "[green]"    // Types
	case "167": return "[red]"      // Errors
	
	// 216-color cube (16-231) - approximate mapping
	default:
		// For other colors, try to map to closest basic color
		switch {
		case colorNum >= "16" && colorNum <= "21":   // Dark blues
			return "[blue]"
		case colorNum >= "22" && colorNum <= "51":   // Greens
			return "[green]"
		case colorNum >= "52" && colorNum <= "87":   // Yellows/oranges
			return "[yellow]"
		case colorNum >= "88" && colorNum <= "123":  // Reds
			return "[red]"
		case colorNum >= "124" && colorNum <= "159": // Magentas
			return "[magenta]"
		case colorNum >= "160" && colorNum <= "195": // Cyans
			return "[cyan]"
		case colorNum >= "196" && colorNum <= "231": // Whites/grays
			return "[white]"
		case colorNum >= "232" && colorNum <= "255": // Grayscale
			return "[dim]"
		default:
			return "[white]" // Fallback
		}
	}
}

// formatJSON formats and highlights JSON content with enhanced formatting
func (f *ContentFormatter) formatJSON(content string) string {
	var jsonData interface{}
	if json.Unmarshal([]byte(content), &jsonData) == nil {
		formatted, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return content
		}
		
		// Use Chroma for JSON highlighting
		return f.formatWithChroma(string(formatted), "json")
	}
	return content
}

// formatHTML formats and highlights HTML content
func (f *ContentFormatter) formatHTML(content string) string {
	formatted := gohtml.Format(content)
	return f.formatWithChroma(formatted, "html")
}

// formatXML formats and highlights XML content
func (f *ContentFormatter) formatXML(content string) string {
	formatted := xmlfmt.FormatXML(content, "", "  ")
	return f.formatWithChroma(formatted, "xml")
}

// createSideBySideLayout creates a simple fallback layout - real layout should be handled by tview
func (f *ContentFormatter) createSideBySideLayout(leftTitle, leftContent, rightTitle, rightContent string) string {
	// Return a marker that indicates this content needs tview native layout
	// Use a delimiter that's unlikely to appear in content: |||SPLIT|||
	// The UI layer should detect this and create proper tview panes
	return fmt.Sprintf("TVIEW_LAYOUT:%s|||SPLIT|||%s|||SPLIT|||%s|||SPLIT|||%s", leftTitle, rightTitle, leftContent, rightContent)
}

// truncateAndPadLine truncates long lines and pads short ones to a fixed width
// while preserving tview color codes
func (f *ContentFormatter) truncateAndPadLine(line string, width int) string {
	// Calculate visible length (excluding tview color codes)
	visibleLen := f.calculateVisibleLength(line)
	
	// If the line is too long, truncate it
	if visibleLen > width {
		// For hex content, truncate more aggressively to ensure it fits
		if strings.Contains(line, ":") && strings.Contains(line, "|") {
			// This looks like hex content, truncate without "..." to maximize data shown
			result := f.truncateLineToWidth(line, width)
			// Ensure exact width
			resultLen := f.calculateVisibleLength(result)
			if resultLen < width {
				result += strings.Repeat(" ", width-resultLen)
			}
			return result
		} else {
			truncated := f.truncateLineToWidth(line, width-3) + "..."
			// Ensure exact width
			truncatedLen := f.calculateVisibleLength(truncated)
			if truncatedLen < width {
				truncated += strings.Repeat(" ", width-truncatedLen)
			}
			return truncated
		}
	}
	
	// If the line is too short, pad it with spaces to exact width
	if visibleLen < width {
		padding := width - visibleLen
		return line + strings.Repeat(" ", padding)
	}
	
	// If exactly the right length
	return line
}

// calculateVisibleLength calculates the visible length of a string, ignoring tview color codes
func (f *ContentFormatter) calculateVisibleLength(s string) int {
	// Remove tview color codes like [color] and [#ffffff] and [-]
	tviewColorPattern := regexp.MustCompile(`\[[^\]]*\]`)
	cleaned := tviewColorPattern.ReplaceAllString(s, "")
	return len(cleaned)
}

// truncateLineToWidth truncates a line to a specific visible width while preserving color codes
func (f *ContentFormatter) truncateLineToWidth(line string, width int) string {
	if width <= 0 {
		return ""
	}
	
	var result strings.Builder
	visibleCount := 0
	runes := []rune(line)
	
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		
		if r == '[' {
			// Look for closing bracket to see if this is a color code
			closeBracketPos := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == ']' {
					closeBracketPos = j
					break
				}
			}
			
			if closeBracketPos != -1 {
				// This is a color code, include it without counting towards visible length
				for k := i; k <= closeBracketPos; k++ {
					result.WriteRune(runes[k])
				}
				i = closeBracketPos // Skip past the color code (loop will increment)
				continue
			}
		}
		
		// Regular character - count towards visible length
		if visibleCount >= width {
			// Before breaking, ensure we close any active color codes
			// Add a reset to prevent bleeding
			if strings.Contains(result.String(), "[") && !strings.HasSuffix(result.String(), "[-]") {
				result.WriteString("[-]")
			}
			break
		}
		
		result.WriteRune(r)
		visibleCount++
	}
	
	// Final safety check: ensure the result doesn't end with incomplete color codes
	resultStr := result.String()
	
	// Check if we have unclosed color brackets
	lastOpenBracket := strings.LastIndex(resultStr, "[")
	lastCloseBracket := strings.LastIndex(resultStr, "]")
	
	if lastOpenBracket > lastCloseBracket && lastOpenBracket != -1 {
		// We have an unclosed color code - need to fix this
		// Find where the incomplete code starts and truncate there
		resultRunes := []rune(resultStr)
		for i := lastOpenBracket; i < len(resultRunes); i++ {
			if resultRunes[i] == ']' {
				// Found the close - we're okay
				break
			}
			if i == len(resultRunes)-1 {
				// Never found close bracket, truncate at the open bracket and add reset
				resultStr = string(resultRunes[:lastOpenBracket]) + "[-]"
				break
			}
		}
	}
	
	return resultStr
}

// getTerminalWidth gets the current terminal width
func (f *ContentFormatter) getTerminalWidth() int {
	// Try to get terminal size from stdin
	if width, _, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
		return width
	}
	
	// Try stderr as fallback
	if width, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil {
		return width
	}
	
	// Default fallback width
	return 120
}

// ensureColorReset ensures a line ends with a color reset to prevent bleeding
func (f *ContentFormatter) ensureColorReset(line string) string {
	if line == "" {
		return line
	}
	
	// For image content, always add a strong reset to prevent bleeding
	// This is especially important for complex image content with many colors
	if strings.Contains(line, "▄") || strings.Contains(line, "█") || strings.Contains(line, "▀") {
		// This looks like image block content, ensure strong reset
		return strings.TrimRight(line, " ") + "[-]"
	}
	
	// Check if the line already ends with a color reset or white color
	if strings.HasSuffix(line, "[-]") || strings.HasSuffix(line, "[white]") {
		return line
	}
	
	// Add a color reset at the end
	return line + "[-]"
}

// forceColorReset strips any trailing color codes and ensures clean termination
func (f *ContentFormatter) forceColorReset(line string) string {
	if line == "" {
		return line
	}
	
	// Remove any existing trailing color codes to avoid conflicts
	line = strings.TrimRight(line, " ")
	
	// For image content, be extra aggressive about resetting
	if strings.Contains(line, "▄") || strings.Contains(line, "█") || strings.Contains(line, "▀") {
		// Strip any trailing incomplete color codes and add definitive reset
		if strings.HasSuffix(line, "[-]") {
			return line // Already properly terminated
		}
		return line // The explicit reset will be added in the format string
	}
	
	return line
}

// ensureLineBoundary ensures clean line boundaries without destroying existing colors
func (f *ContentFormatter) ensureLineBoundary(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	
	// Only ensure the line doesn't end with dangling color codes that could bleed
	// Preserve all existing colors, just make sure they don't cross boundaries
	line = strings.TrimRight(line, " ")
	
	// If line ends with an incomplete color code (like [#ff8400 without closing ),
	// we need to be more careful, but since truncateAndPadLine should handle this,
	// we just ensure there's no active color state bleeding
	if !strings.HasSuffix(line, "[-]") && !strings.HasSuffix(line, "[white]") {
		// Only add boundary if we detect active color content
		if strings.Contains(line, "[") && strings.LastIndex(line, "[") > strings.LastIndex(line, "]") {
			// There's an unclosed color tag - add boundary
			return line + "[-]"
		}
	}
	
	return line
}

// extractImageContent extracts just the image content, removing header information
func (f *ContentFormatter) extractImageContent(fullOutput string) string {
	lines := strings.Split(fullOutput, "\n")
	
	// Skip the first few lines that contain headers like:
	// "Image Preview (WebP, 360x450, ANSI Block Characters)"
	// and any empty lines after it
	startIdx := 0
	for i, line := range lines {
		// Look for lines that don't contain header info patterns
		if !strings.Contains(line, "Image Preview") &&
		   !strings.Contains(line, "PNG") &&
		   !strings.Contains(line, "JPEG") &&
		   !strings.Contains(line, "WebP") &&
		   !strings.Contains(line, "GIF") &&
		   !strings.Contains(line, "BMP") &&
		   !strings.Contains(line, "ANSI Block Characters") &&
		   !strings.Contains(line, "Kitty Graphics") &&
		   !strings.Contains(line, "iTerm2 Inline") &&
		   !strings.Contains(line, "Sixel Protocol") &&
		   strings.TrimSpace(line) != "" {
			startIdx = i
			break
		}
	}
	
	// Join the remaining lines
	if startIdx < len(lines) {
		return strings.Join(lines[startIdx:], "\n")
	}
	
	// If we couldn't find content, return the original (might be an error message)
	return fullOutput
}

// formatCompactHexPreview creates a compact hex dump suitable for side-by-side layout
func (f *ContentFormatter) formatCompactHexPreview(content string) string {
	data := []byte(content)
	maxBytes := 256 // Smaller limit for side-by-side view
	
	if len(data) == 0 {
		return "[dim]No binary data[white]"
	}
	
	// Truncate if too large
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	
	var result strings.Builder
	
	// Compact hex dump (8 bytes per line for narrower columns)
	for i := 0; i < len(data); i += 8 {
		// Offset (shorter format)
		result.WriteString(fmt.Sprintf("[blue]%04x:[white] ", i))
		
		// Hex bytes
		for j := 0; j < 8 && i+j < len(data); j++ {
			result.WriteString(fmt.Sprintf("[cyan]%02x[white] ", data[i+j]))
		}
		
		// Pad if incomplete line
		for j := len(data) - i; j < 8 && j >= 0; j++ {
			if i+j >= len(data) {
				result.WriteString("   ") // 3 spaces for missing byte
			}
		}
		
		// ASCII representation (compact)
		result.WriteString("[dim]|[white]")
		for j := 0; j < 8 && i+j < len(data); j++ {
			b := data[i+j]
			if b >= 32 && b <= 126 {
				result.WriteString(fmt.Sprintf("[green]%c[white]", b))
			} else {
				result.WriteString("[dim].[white]")
			}
		}
		result.WriteString("[dim]|[white]")
		
		// Add newline if not the last line
		if i+8 < len(data) {
			result.WriteString("\n")
		}
	}
	
	// Add footer if truncated
	if len([]byte(content)) > maxBytes {
		remaining := len([]byte(content)) - maxBytes
		result.WriteString(fmt.Sprintf("\n[dim]... and %d more bytes[white]", remaining))
	}
	
	return result.String()
}

// extractHexContent extracts just the hex content, removing header information
func (f *ContentFormatter) extractHexContent(fullOutput string) string {
	lines := strings.Split(fullOutput, "\n")
	
	// Skip header lines that contain "Binary Data Preview" and similar
	startIdx := 0
	endIdx := len(lines)
	
	for i, line := range lines {
		// Look for the first line that looks like a hex dump (starts with address)
		if strings.Contains(line, ":") && 
		   (strings.Contains(line, " |") || len(line) > 20) &&
		   !strings.Contains(line, "Binary Data Preview") {
			startIdx = i
			break
		}
	}
	
	// Find the end (before any "... and X more bytes" footer)
	for i := startIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "... and") && strings.Contains(lines[i], "more bytes") {
			endIdx = i + 1 // Include the footer line
			break
		}
	}
	
	// Join the hex dump lines
	if startIdx < endIdx {
		return strings.Join(lines[startIdx:endIdx], "\n")
	}
	
	// If we couldn't find hex content, return the original
	return fullOutput
}

// GetImageDisplayer returns the internal image displayer for testing
func (f *ContentFormatter) GetImageDisplayer() *ImageDisplayer {
	return f.getImageDisplayer()
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
	return f.formatWithChroma(string(pretty), "json")
}