package filter

import (
	"net/url"
	"sort"
	"strings"

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