package filter

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cnharrison/har-tui/internal/har"
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

// FilterEntries filters HAR entries based on current filter state
func (f *FilterState) FilterEntries(entries []har.HAREntry) []int {
	var filteredEntries []int
	
	for i, entry := range entries {
		// Apply error filter
		if f.ShowErrorsOnly && entry.Response.Status < 400 {
			continue
		}
		
		// Apply text filter
		if f.FilterText != "" {
			u, _ := url.Parse(entry.Request.URL)
			hostPath := u.Host + u.Path
			if !strings.Contains(strings.ToLower(hostPath), strings.ToLower(f.FilterText)) {
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
	return []string{"all", "fetch", "doc", "css", "js", "img", "media", "manifest", "ws", "wasm", "other"}
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