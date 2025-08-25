package har

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// LoadHARFile loads and parses a HAR file from the given path
func LoadHARFile(filePath string) (*HARFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var harFile HARFile
	if err := json.Unmarshal(data, &harFile); err != nil {
		return nil, err
	}

	return &harFile, nil
}

// DecodeBase64 decodes base64 content if encoded
func DecodeBase64(text, encoding string) string {
	if encoding == "base64" && text != "" {
		if decoded, err := base64.StdEncoding.DecodeString(text); err == nil {
			return string(decoded)
		}
	}
	return text
}

// ExtractIP extracts IP address from URL if present
func ExtractIP(urlStr string) string {
	if u, err := url.Parse(urlStr); err == nil {
		host := u.Host
		if strings.Contains(host, ":") {
			host, _, _ = net.SplitHostPort(host)
		}
		if net.ParseIP(host) != nil {
			return host
		}
	}
	return ""
}

// GetRequestType determines the request type based on URL and headers
func GetRequestType(entry HAREntry) string {
	// Check for CORS errors first (highest priority)
	if IsCORSError(entry) {
		return "cors"
	}
	
	u, err := url.Parse(entry.Request.URL)
	if err != nil {
		return "other"
	}
	
	path := strings.ToLower(u.Path)
	
	// Check for WebSocket
	if u.Scheme == "ws" || u.Scheme == "wss" {
		return "ws"
	}
	
	// Check Chrome's _resourceType field first (most reliable)
	if entry.ResourceType != "" {
		switch strings.ToLower(entry.ResourceType) {
		case "image":
			return "img"
		case "stylesheet":
			return "css"
		case "script":
			return "js"
		case "document":
			return "doc"
		case "media":
			return "media"
		case "manifest":
			return "manifest"
		case "websocket":
			return "ws"
		case "fetch", "xhr":
			return "fetch"
		case "wasm":
			return "wasm"
		}
	}
	
	// Check content type from response
	for _, header := range entry.Response.Headers {
		if strings.ToLower(header.Name) == "content-type" {
			contentType := strings.ToLower(header.Value)
			switch {
			case strings.Contains(contentType, "text/html"):
				return "doc"
			case strings.Contains(contentType, "text/css"):
				return "css" 
			case strings.Contains(contentType, "javascript") || strings.Contains(contentType, "ecmascript"):
				return "js"
			case strings.Contains(contentType, "image/"):
				return "img"
			case strings.Contains(contentType, "audio/") || strings.Contains(contentType, "video/"):
				return "media"
			case strings.Contains(contentType, "application/manifest") || strings.Contains(contentType, "text/cache-manifest"):
				return "manifest"
			case strings.Contains(contentType, "application/wasm"):
				return "wasm"
			case strings.Contains(contentType, "application/json") || strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml"):
				return "fetch" // API calls
			}
		}
	}
	
	// Check by file extension
	switch {
	case strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".htm"):
		return "doc"
	case strings.HasSuffix(path, ".css"):
		return "css"
	case strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".mjs"):
		return "js"
	case strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") || 
		 strings.HasSuffix(path, ".gif") || strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".webp") ||
		 strings.HasSuffix(path, ".ico"):
		return "img"
	case strings.HasSuffix(path, ".mp4") || strings.HasSuffix(path, ".webm") || strings.HasSuffix(path, ".ogg") ||
		 strings.HasSuffix(path, ".mp3") || strings.HasSuffix(path, ".wav") || strings.HasSuffix(path, ".flac"):
		return "media"
	case strings.HasSuffix(path, ".wasm"):
		return "wasm"
	case strings.HasSuffix(path, ".manifest") || strings.HasSuffix(path, ".webmanifest"):
		return "manifest"
	}
	
	// Check for XHR/Fetch indicators
	for _, header := range entry.Request.Headers {
		headerName := strings.ToLower(header.Name)
		headerValue := strings.ToLower(header.Value)
		if headerName == "x-requested-with" && headerValue == "xmlhttprequest" {
			return "fetch"
		}
		if headerName == "accept" && (strings.Contains(headerValue, "application/json") || strings.Contains(headerValue, "application/xml")) {
			return "fetch"
		}
	}
	
	// Default to doc for root paths or fetch for API-like paths
	if path == "/" || path == "" {
		return "doc"
	}
	if strings.Contains(path, "/api/") || strings.Contains(path, "/rest/") || strings.Contains(path, "/graphql") {
		return "fetch"
	}
	
	return "other"
}

// GenerateDescriptiveFilename creates a descriptive filename based on entry data
func GenerateDescriptiveFilename(entry HAREntry, suffix string) string {
	u, err := url.Parse(entry.Request.URL)
	if err != nil {
		return "request_" + entry.Request.Method + "_unknown" + suffix
	}
	
	// Clean up hostname for filename
	hostname := strings.ReplaceAll(u.Host, ":", "_")
	hostname = regexp.MustCompile(`[^\w\-_.]`).ReplaceAllString(hostname, "_")
	
	// Clean up path for filename
	path := u.Path
	if path == "/" || path == "" {
		path = "root"
	} else {
		path = strings.TrimPrefix(path, "/")
		path = strings.ReplaceAll(path, "/", "_")
		// Remove or replace invalid filename characters
		path = regexp.MustCompile(`[^\w\-_.]`).ReplaceAllString(path, "_")
		// Truncate if too long
		if len(path) > 50 {
			path = path[:50]
		}
	}
	
	// Get request type for additional context
	requestType := GetRequestType(entry)
	
	// Combine into descriptive filename
	filename := entry.Request.Method + "_" + hostname + "_" + path + "_" + requestType + suffix
	
	// Clean up any double underscores
	filename = regexp.MustCompile(`_+`).ReplaceAllString(filename, "_")
	
	return filename
}

// IsCORSError determines if a request failed due to CORS issues
func IsCORSError(entry HAREntry) bool {
	// Only check status 0 (blocked) requests
	if entry.Response.Status != 0 {
		return false
	}
	
	method := strings.ToUpper(entry.Request.Method)
	
	// 1. Failed preflights: OPTIONS with Access-Control-Request-Method
	if method == "OPTIONS" {
		for _, header := range entry.Request.Headers {
			if strings.ToLower(header.Name) == "access-control-request-method" {
				return true // This is a failed CORS preflight
			}
		}
		return false
	}
	
	// 2. Check for explicit CORS indicators
	for _, header := range entry.Request.Headers {
		headerName := strings.ToLower(header.Name)
		headerValue := strings.ToLower(header.Value)
		
		// Origin header indicates cross-origin request
		if headerName == "origin" {
			return true
		}
		
		// Sec-Fetch-Mode: cors indicates CORS request
		if headerName == "sec-fetch-mode" && headerValue == "cors" {
			return true
		}
	}
	
	// 3. Check for the specific flatfile pattern: 
	//    Cross-subdomain + Content-Type: application/json on GET (triggers preflight)
	if method == "GET" {
		var hasCrossOrigin, hasContentTypeJson bool
		
		// Parse request URL
		requestURL, err := url.Parse(entry.Request.URL)
		if err != nil {
			return false
		}
		requestHost := requestURL.Host
		
		// Check headers
		for _, header := range entry.Request.Headers {
			headerName := strings.ToLower(header.Name)
			headerValue := strings.ToLower(header.Value)
			
			// Cross-origin via Referer
			if headerName == "referer" {
				if refererURL, err := url.Parse(header.Value); err == nil {
					refererHost := refererURL.Host
					if requestHost != "" && refererHost != "" && requestHost != refererHost {
						hasCrossOrigin = true
					}
				}
			}
			
			// Content-Type: application/json triggers preflight on GET
			if headerName == "content-type" && strings.Contains(headerValue, "application/json") {
				hasContentTypeJson = true
			}
		}
		
		// This specific combination is almost certainly a CORS issue
		if hasCrossOrigin && hasContentTypeJson {
			return true
		}
	}
	
	return false
}

// SaveFilteredHAR saves filtered HAR entries to a new file
func SaveFilteredHAR(originalHAR *HARFile, filteredIndices []int, outputPath string) error {
	// Create a new HAR file with only filtered entries
	filteredHAR := &HARFile{
		Log: HARLog{
			Version: originalHAR.Log.Version,
			Entries: make([]HAREntry, 0, len(filteredIndices)),
		},
	}
	
	// Copy only the filtered entries
	for _, idx := range filteredIndices {
		if idx >= 0 && idx < len(originalHAR.Log.Entries) {
			filteredHAR.Log.Entries = append(filteredHAR.Log.Entries, originalHAR.Log.Entries[idx])
		}
	}
	
	// Marshal to JSON with proper formatting
	data, err := json.MarshalIndent(filteredHAR, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file
	return os.WriteFile(outputPath, data, 0644)
}