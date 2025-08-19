package filter

import (
	"testing"

	"github.com/cnharrison/har-tui/internal/har"
)

func createTestEntry(method, url string, status int, mimeType string) har.HAREntry {
	var headers []har.HARHeader
	if mimeType != "" {
		headers = []har.HARHeader{
			{Name: "Content-Type", Value: mimeType},
		}
	}
	
	return har.HAREntry{
		Request: har.HARRequest{
			Method: method,
			URL:    url,
		},
		Response: har.HARResponse{
			Status: status,
			Headers: headers,
			Content: har.HARContent{
				MimeType: mimeType,
			},
		},
		Time: 100.0,
		Timings: har.HARTimings{}, // Add empty timings to avoid issues
	}
}

func TestFilterState_FilterEntries(t *testing.T) {
	entries := []har.HAREntry{
		createTestEntry("GET", "https://api.example.com/users", 200, "application/json"),
		createTestEntry("POST", "https://api.example.com/posts", 201, "application/json"),
		createTestEntry("GET", "https://cdn.example.com/image.jpg", 200, "image/jpeg"),
		createTestEntry("GET", "https://api.example.com/error", 500, "text/html"),
		createTestEntry("PUT", "https://other.com/update", 204, ""),
	}

	tests := []struct {
		name     string
		setup    func(*FilterState)
		expected []int
	}{
		{
			name: "no filters",
			setup: func(fs *FilterState) {
				// default state
			},
			expected: []int{0, 1, 2, 3, 4},
		},
		{
			name: "text filter by host",
			setup: func(fs *FilterState) {
				fs.SetTextFilter("api.example.com")
			},
			expected: []int{0, 1, 3},
		},
		{
			name: "text filter by path",
			setup: func(fs *FilterState) {
				fs.SetTextFilter("users")
			},
			expected: []int{0},
		},
		{
			name: "errors only",
			setup: func(fs *FilterState) {
				fs.ToggleErrorsOnly()
			},
			expected: []int{3},
		},
		{
			name: "type filter - fetch (JSON)",
			setup: func(fs *FilterState) {
				fs.SetTypeFilter("fetch")
			},
			expected: []int{0, 1}, // JSON responses are considered "fetch"
		},
		{
			name: "type filter - img",
			setup: func(fs *FilterState) {
				fs.SetTypeFilter("img")
			},
			expected: []int{2},
		},
		{
			name: "combined filters - api host and errors",
			setup: func(fs *FilterState) {
				fs.SetTextFilter("api.example.com")
				fs.ToggleErrorsOnly()
			},
			expected: []int{3},
		},
		{
			name: "sort by slowest (already slow to fast)",
			setup: func(fs *FilterState) {
				fs.ToggleSortBySlowest()
			},
			expected: []int{0, 1, 2, 3, 4}, // Same order since all have same time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFilterState()
			tt.setup(fs)
			
			result := fs.FilterEntries(entries)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d results, got %d", len(tt.expected), len(result))
				return
			}
			
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected index %d at position %d, got %d", expected, i, result[i])
				}
			}
		})
	}
}

func TestFilterState_SetTextFilter(t *testing.T) {
	fs := NewFilterState()
	
	// Test setting filter
	fs.SetTextFilter("test")
	if fs.FilterText != "test" {
		t.Errorf("Expected FilterText 'test', got %q", fs.FilterText)
	}
	
	// Test clearing filter
	fs.SetTextFilter("")
	if fs.FilterText != "" {
		t.Errorf("Expected empty FilterText, got %q", fs.FilterText)
	}
}

func TestFilterState_ToggleMethods(t *testing.T) {
	fs := NewFilterState()
	
	// Test toggle errors only
	if fs.ShowErrorsOnly {
		t.Error("Expected ShowErrorsOnly to be false initially")
	}
	fs.ToggleErrorsOnly()
	if !fs.ShowErrorsOnly {
		t.Error("Expected ShowErrorsOnly to be true after toggle")
	}
	fs.ToggleErrorsOnly()
	if fs.ShowErrorsOnly {
		t.Error("Expected ShowErrorsOnly to be false after second toggle")
	}
	
	// Test toggle sort by slowest
	if fs.SortBySlowest {
		t.Error("Expected SortBySlowest to be false initially")
	}
	fs.ToggleSortBySlowest()
	if !fs.SortBySlowest {
		t.Error("Expected SortBySlowest to be true after toggle")
	}
}

func TestFilterState_TypeFilter(t *testing.T) {
	fs := NewFilterState()
	
	// Test initial state
	if fs.ActiveTypeFilter != "all" {
		t.Errorf("Expected initial ActiveTypeFilter 'all', got %q", fs.ActiveTypeFilter)
	}
	
	// Test setting type filter
	fs.SetTypeFilter("js")
	if fs.ActiveTypeFilter != "js" {
		t.Errorf("Expected ActiveTypeFilter 'js', got %q", fs.ActiveTypeFilter)
	}
	
	// Test reset
	fs.Reset()
	if fs.ActiveTypeFilter != "all" {
		t.Errorf("Expected ActiveTypeFilter 'all' after reset, got %q", fs.ActiveTypeFilter)
	}
	if fs.FilterText != "" {
		t.Errorf("Expected empty FilterText after reset, got %q", fs.FilterText)
	}
	if fs.ShowErrorsOnly {
		t.Error("Expected ShowErrorsOnly false after reset")
	}
	if fs.SortBySlowest {
		t.Error("Expected SortBySlowest false after reset")
	}
}

func TestGetTypeFilters(t *testing.T) {
	filters := GetTypeFilters()
	
	expected := []string{"all", "fetch", "doc", "css", "js", "img", "media", "manifest", "ws", "wasm", "other"}
	
	if len(filters) != len(expected) {
		t.Errorf("Expected %d type filters, got %d", len(expected), len(filters))
		return
	}
	
	for i, expectedFilter := range expected {
		if filters[i] != expectedFilter {
			t.Errorf("Expected filter %q at position %d, got %q", expectedFilter, i, filters[i])
		}
	}
}