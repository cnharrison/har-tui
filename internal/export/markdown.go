package export

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cnharrison/har-tui/internal/har"
)

// GenerateMarkdownSummary generates a detailed markdown report for support/debugging
func GenerateMarkdownSummary(entry har.HAREntry) string {
	u, _ := url.Parse(entry.Request.URL)
	
	// Parse timestamp for better date display
	timestamp, err := time.Parse("2006-01-02T15:04:05.000Z", entry.StartedDateTime)
	if err != nil {
		timestamp, err = time.Parse(time.RFC3339, entry.StartedDateTime)
		if err != nil {
			timestamp = time.Now()
		}
	}
	
	var summary strings.Builder
	
	// Title with quick status indicator
	statusEmoji := "✅"
	if entry.Response.Status >= 500 {
		statusEmoji = "🔥" // Server error
	} else if entry.Response.Status >= 400 {
		statusEmoji = "⚠️"  // Client error
	} else if entry.Response.Status >= 300 {
		statusEmoji = "↩️"  // Redirect
	}
	
	summary.WriteString(fmt.Sprintf("# %s %s %d - %s Request Issue\n\n", statusEmoji, entry.Request.Method, entry.Response.Status, u.Host))
	
	// Critical info first (what support engineers need immediately)
	summary.WriteString("## 🚨 Critical Info\n\n")
	summary.WriteString(fmt.Sprintf("- **Date/Time:** %s (%s UTC)\n", timestamp.Format("Monday, January 2, 2006 at 3:04:05 PM"), timestamp.Format("2006-01-02 15:04:05")))
	summary.WriteString(fmt.Sprintf("- **Status:** %d %s\n", entry.Response.Status, entry.Response.StatusText))
	summary.WriteString(fmt.Sprintf("- **Response Time:** %.0fms", entry.Time))
	if entry.Time > 5000 {
		summary.WriteString(" ⚠️ SLOW")
	} else if entry.Time > 2000 {
		summary.WriteString(" 🐌 Sluggish")
	}
	summary.WriteString("\n")
	summary.WriteString(fmt.Sprintf("- **Method:** %s\n", entry.Request.Method))
	summary.WriteString(fmt.Sprintf("- **Host:** %s\n", u.Host))
	summary.WriteString(fmt.Sprintf("- **Path:** %s\n\n", u.Path))
	
	// Full URL for easy copy/paste
	summary.WriteString("## 🔗 Request Details\n\n")
	summary.WriteString(fmt.Sprintf("**Full URL:** `%s`\n\n", entry.Request.URL))
	
	// Important headers only (filter noise)
	importantHeaders := []string{"authorization", "content-type", "accept", "user-agent", "x-", "cookie", "auth"}
	var reqHeaders []har.HARHeader
	for _, header := range entry.Request.Headers {
		headerLower := strings.ToLower(header.Name)
		for _, important := range importantHeaders {
			if strings.Contains(headerLower, important) {
				reqHeaders = append(reqHeaders, header)
				break
			}
		}
	}
	
	if len(reqHeaders) > 0 {
		summary.WriteString("## 📋 Key Request Headers\n\n")
		for _, header := range reqHeaders {
			// Redact sensitive information
			value := header.Value
			if strings.ToLower(header.Name) == "authorization" || strings.Contains(strings.ToLower(header.Name), "auth") {
				if len(value) > 10 {
					value = value[:10] + "..." + value[len(value)-4:] + " (redacted)"
				}
			}
			summary.WriteString(fmt.Sprintf("- **%s:** `%s`\n", header.Name, value))
		}
		summary.WriteString("\n")
	}
	
	// Request body (if exists and relevant)
	if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
		summary.WriteString("## 📤 Request Body\n\n")
		bodyText := entry.Request.PostData.Text
		
		// Format based on content type
		var lang string
		mimeType := strings.ToLower(entry.Request.PostData.MimeType)
		switch {
		case strings.Contains(mimeType, "json"):
			lang = "json"
		case strings.Contains(mimeType, "xml"):
			lang = "xml"
		case strings.Contains(mimeType, "html"):
			lang = "html"
		case strings.Contains(mimeType, "css"):
			lang = "css"
		default:
			lang = "text"
		}
		
		// Truncate if too long but keep important parts
		if len(bodyText) > 500 {
			summary.WriteString(fmt.Sprintf("```%s\n%s\n... (showing first 500 chars of %d total)\n```\n\n", lang, bodyText[:500], len(bodyText)))
		} else {
			summary.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, bodyText))
		}
	}
	
	// Response info
	summary.WriteString("## 📥 Response Info\n\n")
	
	// Key response headers
	var respHeaders []har.HARHeader
	responseImportant := []string{"content-type", "content-length", "cache-control", "set-cookie", "location", "server", "x-"}
	for _, header := range entry.Response.Headers {
		headerLower := strings.ToLower(header.Name)
		for _, important := range responseImportant {
			if strings.Contains(headerLower, important) {
				respHeaders = append(respHeaders, header)
				break
			}
		}
	}
	
	if len(respHeaders) > 0 {
		summary.WriteString("**Key Headers:**\n")
		for _, header := range respHeaders {
			summary.WriteString(fmt.Sprintf("- **%s:** `%s`\n", header.Name, header.Value))
		}
		summary.WriteString("\n")
	}
	
	// Response body (error responses especially)
	if entry.Response.Content.Text != "" {
		bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
		if entry.Response.Status >= 400 || strings.Contains(strings.ToLower(bodyText), "error") {
			summary.WriteString("**Error Response:**\n")
		} else {
			summary.WriteString("**Response Body:**\n")
		}
		
		var lang string
		mimeType := strings.ToLower(entry.Response.Content.MimeType)
		switch {
		case strings.Contains(mimeType, "json"):
			lang = "json"
		case strings.Contains(mimeType, "xml"):
			lang = "xml"
		case strings.Contains(mimeType, "html"):
			lang = "html"
		default:
			lang = "text"
		}
		
		// For errors, show more content
		maxLen := 800
		if entry.Response.Status >= 400 {
			maxLen = 1500
		}
		
		if len(bodyText) > maxLen {
			summary.WriteString(fmt.Sprintf("```%s\n%s\n... (showing first %d chars of %d total)\n```\n\n", lang, bodyText[:maxLen], maxLen, len(bodyText)))
		} else {
			summary.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, bodyText))
		}
	}
	
	// Performance breakdown (only if slow or there are issues)
	if entry.Time > 1000 || entry.Response.Status >= 400 {
		summary.WriteString("## ⏱️ Performance Breakdown\n\n")
		summary.WriteString(fmt.Sprintf("- **DNS:** %.0fms\n", entry.Timings.DNS))
		summary.WriteString(fmt.Sprintf("- **Connect:** %.0fms\n", entry.Timings.Connect))
		if entry.Timings.SSL > 0 {
			summary.WriteString(fmt.Sprintf("- **SSL:** %.0fms\n", entry.Timings.SSL))
		}
		summary.WriteString(fmt.Sprintf("- **Send:** %.0fms\n", entry.Timings.Send))
		summary.WriteString(fmt.Sprintf("- **Wait (TTFB):** %.0fms\n", entry.Timings.Wait))
		summary.WriteString(fmt.Sprintf("- **Receive:** %.0fms\n", entry.Timings.Receive))
		summary.WriteString(fmt.Sprintf("- **Total:** %.0fms\n\n", entry.Time))
	}
	
	// Quick troubleshooting hints
	if entry.Response.Status >= 400 {
		summary.WriteString("## 🔧 Quick Troubleshooting\n\n")
		switch {
		case entry.Response.Status == 401:
			summary.WriteString("- Check authentication headers/tokens\n- Verify API keys are valid\n- Check token expiration\n")
		case entry.Response.Status == 403:
			summary.WriteString("- Check user permissions\n- Verify resource access rights\n- Check rate limiting\n")
		case entry.Response.Status == 404:
			summary.WriteString("- Verify URL path is correct\n- Check if resource exists\n- Validate route configuration\n")
		case entry.Response.Status >= 500:
			summary.WriteString("- Server-side issue\n- Check server logs\n- Verify service health\n")
		case entry.Response.Status == 429:
			summary.WriteString("- Rate limiting active\n- Check retry-after header\n- Implement backoff strategy\n")
		}
		summary.WriteString("\n")
	}
	
	summary.WriteString("---\n*Generated by HAR-TUI for support investigation*")
	
	return summary.String()
}