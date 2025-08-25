package filter

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cnharrison/har-tui/internal/har"
	"github.com/cnharrison/har-tui/internal/util"
)

// FilterState holds the current filtering state
type FilterState struct {
	FilterText       string
	ShowErrorsOnly   bool
	SortBySlowest    bool
	ActiveTypeFilter string
}

// NewFilterState creates a new filter state
func NewFilterState() *FilterState {
	return &FilterState{
		FilterText:       "",
		ShowErrorsOnly:   false,
		SortBySlowest:    false,
		ActiveTypeFilter: "all",
	}
}

// FilterEntries filters HAR entries based on current filter state (legacy O(n) method)
func (f *FilterState) FilterEntries(entries []har.HAREntry) []int {
	var filteredEntries []int
	
	for i, entry := range entries {
		// Apply error filter
		if f.ShowErrorsOnly && entry.Response.Status < 400 && entry.Response.Status != 0 {
			continue
		}
		
		// Apply text filter
		if f.FilterText != "" {
			if !f.matchesTextSearch(entry, strings.ToLower(f.FilterText)) {
				continue
			}
		}
		
		// Apply type filter
		if f.ActiveTypeFilter != "all" {
			requestType := har.GetRequestType(entry)
			if requestType != f.ActiveTypeFilter {
				continue
			}
		}
		
		filteredEntries = append(filteredEntries, i)
	}
	
	// Sort if requested
	if f.SortBySlowest {
		sort.Slice(filteredEntries, func(i, j int) bool {
			return entries[filteredEntries[i]].Time > entries[filteredEntries[j]].Time
		})
	}
	
	return filteredEntries
}

// FilterEntriesWithIndex filters HAR entries using O(1) index lookups
func (f *FilterState) FilterEntriesWithIndex(entries []har.HAREntry, index *har.EntryIndex) []int {
	var result []int
	
	// Start with all entries or type filter
	if f.ActiveTypeFilter == "all" {
		for i := range entries {
			result = append(result, i)
		}
	} else {
		result = index.GetByType(f.ActiveTypeFilter)
	}
	
	// Apply error filter using index
	if f.ShowErrorsOnly {
		errorIndices := index.GetErrorIndices(entries)
		result = util.IntersectIndices(result, errorIndices)
	}
	
	// Apply text filter (still O(n) but only on filtered set)
	if f.FilterText != "" {
		textIndices := index.FilterByText(entries, f.FilterText)
		result = util.IntersectIndices(result, textIndices)
	}
	
	// Sort if requested
	if f.SortBySlowest {
		sort.Slice(result, func(i, j int) bool {
			return entries[result[i]].Time > entries[result[j]].Time
		})
	}
	
	return result
}


// Reset resets all filters to their default state
func (f *FilterState) Reset() {
	f.FilterText = ""
	f.ShowErrorsOnly = false
	f.SortBySlowest = false
	f.ActiveTypeFilter = "all"
}

// ToggleErrorsOnly toggles the errors-only filter
func (f *FilterState) ToggleErrorsOnly() {
	f.ShowErrorsOnly = !f.ShowErrorsOnly
}

// ToggleSortBySlowest toggles sorting by slowest requests
func (f *FilterState) ToggleSortBySlowest() {
	f.SortBySlowest = !f.SortBySlowest
}

// SetTextFilter sets the text filter
func (f *FilterState) SetTextFilter(text string) {
	f.FilterText = text
}

// SetTypeFilter sets the type filter
func (f *FilterState) SetTypeFilter(filterType string) {
	f.ActiveTypeFilter = filterType
}

// GetTypeFilters returns available type filters
func GetTypeFilters() []string {
	return []string{"all", "fetch", "doc", "css", "js", "img", "media", "manifest", "cors", "ws", "wasm", "other"}
}

// matchesTextSearch performs comprehensive text matching across all request/response fields
func (f *FilterState) matchesTextSearch(entry har.HAREntry, searchText string) bool {
	// 1. Search URL (host, path, query parameters)
	if u, err := url.Parse(entry.Request.URL); err == nil {
		if strings.Contains(strings.ToLower(u.Host), searchText) ||
		   strings.Contains(strings.ToLower(u.Path), searchText) ||
		   strings.Contains(strings.ToLower(u.RawQuery), searchText) {
			return true
		}
	}
	
	// 2. Search request method
	if strings.Contains(strings.ToLower(entry.Request.Method), searchText) {
		return true
	}
	
	// 3. Search request headers (names and values)
	for _, header := range entry.Request.Headers {
		if strings.Contains(strings.ToLower(header.Name), searchText) ||
		   strings.Contains(strings.ToLower(header.Value), searchText) {
			return true
		}
	}
	
	// 4. Search response headers (names and values)
	for _, header := range entry.Response.Headers {
		if strings.Contains(strings.ToLower(header.Name), searchText) ||
		   strings.Contains(strings.ToLower(header.Value), searchText) {
			return true
		}
	}
	
	// 5. Search response status text
	if strings.Contains(strings.ToLower(entry.Response.StatusText), searchText) {
		return true
	}
	
	// 6. Search response content type
	if strings.Contains(strings.ToLower(entry.Response.Content.MimeType), searchText) {
		return true
	}
	
	// 7. Search request body (if present and not too large)
	if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
		// Limit body search to reasonable size for performance
		bodyText := entry.Request.PostData.Text
		if len(bodyText) <= 10000 { // 10KB limit for body search
			if strings.Contains(strings.ToLower(bodyText), searchText) {
				return true
			}
		}
	}
	
	// 8. Search response body (if present and not too large)
	if entry.Response.Content.Text != "" {
		// Decode and search response body
		bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
		if len(bodyText) <= 10000 { // 10KB limit for body search
			if strings.Contains(strings.ToLower(bodyText), searchText) {
				return true
			}
		}
	}
	
	// 9. Search cookies (names and values)
	for _, cookie := range entry.Request.Cookies {
		if strings.Contains(strings.ToLower(cookie.Name), searchText) ||
		   strings.Contains(strings.ToLower(cookie.Value), searchText) {
			return true
		}
	}
	
	for _, cookie := range entry.Response.Cookies {
		if strings.Contains(strings.ToLower(cookie.Name), searchText) ||
		   strings.Contains(strings.ToLower(cookie.Value), searchText) {
			return true
		}
	}
	
	return false
}

// GenerateFilteredFilename creates a descriptive filename based on current filters
func (f *FilterState) GenerateFilteredFilename(originalFilename string) string {
	// Get timestamp for uniqueness
	timestamp := time.Now().Format("20060102_150405")
	
	// Remove extension from original filename
	baseName := originalFilename
	if lastDot := strings.LastIndex(baseName, "."); lastDot != -1 {
		baseName = baseName[:lastDot]
	}
	
	// Build filter description parts
	var filterParts []string
	
	// Add type filter
	if f.ActiveTypeFilter != "all" {
		filterParts = append(filterParts, f.ActiveTypeFilter)
	}
	
	// Add text filter (cleaned for filename)
	if f.FilterText != "" {
		cleanedText := regexp.MustCompile(`[^\w\-_.]`).ReplaceAllString(f.FilterText, "_")
		if len(cleanedText) > 20 {
			cleanedText = cleanedText[:20]
		}
		filterParts = append(filterParts, "search_" + cleanedText)
	}
	
	// Add error filter
	if f.ShowErrorsOnly {
		filterParts = append(filterParts, "errors_only")
	}
	
	// Add sort filter
	if f.SortBySlowest {
		filterParts = append(filterParts, "by_slowest")
	}
	
	// Combine all parts
	var filename string
	if len(filterParts) > 0 {
		filterDesc := strings.Join(filterParts, "_")
		filename = fmt.Sprintf("%s_filtered_%s_%s.har", baseName, filterDesc, timestamp)
	} else {
		filename = fmt.Sprintf("%s_all_entries_%s.har", baseName, timestamp)
	}
	
	// Clean up any double underscores
	filename = regexp.MustCompile(`_+`).ReplaceAllString(filename, "_")
	
	return filename
}