package ui

import (
	"testing"
	"time"
)

func TestParseHARDateTime(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		expected    string // expected format for comparison
	}{
		{
			name:        "HAR standard format with milliseconds",
			input:       "2023-01-01T12:00:00.123Z",
			shouldError: false,
			expected:    "2023-01-01T12:00:00.123Z",
		},
		{
			name:        "RFC3339 format",
			input:       "2023-01-01T12:00:00Z",
			shouldError: false,
			expected:    "2023-01-01T12:00:00Z",
		},
		{
			name:        "RFC3339 with timezone",
			input:       "2023-01-01T12:00:00-07:00",
			shouldError: false,
			expected:    "2023-01-01T19:00:00Z", // Converted to UTC
		},
		{
			name:        "HAR format with timezone offset",
			input:       "2023-01-01T12:00:00.123-07:00",
			shouldError: false,
			expected:    "2023-01-01T19:00:00.123Z", // Converted to UTC
		},
		{
			name:        "Invalid format",
			input:       "not-a-valid-date",
			shouldError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHARDateTime(tt.input)
			
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}
			
			// Convert result back to UTC for comparison
			resultUTC := result.UTC()
			expectedTime, err := time.Parse(time.RFC3339, tt.expected)
			if err != nil {
				expectedTime, err = time.Parse("2006-01-02T15:04:05.000Z", tt.expected)
				if err != nil {
					t.Fatalf("Invalid expected time format in test: %q", tt.expected)
				}
			}
			expectedUTC := expectedTime.UTC()
			
			if !resultUTC.Equal(expectedUTC) {
				t.Errorf("Expected %v, got %v", expectedUTC, resultUTC)
			}
		})
	}
}

func TestParseHARDateTimeFormats(t *testing.T) {
	// Test that all supported formats are actually supported
	testTime := time.Date(2023, 1, 1, 12, 0, 0, 123000000, time.UTC)
	
	formatTests := []struct {
		format string
		value  string
	}{
		{"HAR standard", "2023-01-01T12:00:00.123Z"},
		{"RFC3339Nano", testTime.Format(time.RFC3339Nano)},
		{"RFC3339", testTime.Format(time.RFC3339)},
		{"HAR without milliseconds", "2023-01-01T12:00:00Z"},
	}
	
	for _, tt := range formatTests {
		t.Run(tt.format, func(t *testing.T) {
			_, err := parseHARDateTime(tt.value)
			if err != nil {
				t.Errorf("Failed to parse %s format %q: %v", tt.format, tt.value, err)
			}
		})
	}
}

func TestWaterfallViewGetSelectedIndex(t *testing.T) {
	wv := NewWaterfallView()
	
	// Test with empty indices
	if index := wv.GetSelectedIndex(); index != -1 {
		t.Errorf("Expected -1 for empty indices, got %d", index)
	}
	
	// Test with valid indices
	wv.indices = []int{5, 10, 15}
	wv.listView.AddItem("Item 1", "", 0, nil)
	wv.listView.AddItem("Item 2", "", 0, nil) 
	wv.listView.AddItem("Item 3", "", 0, nil)
	wv.listView.SetCurrentItem(1)
	
	expected := 10 // indices[1]
	if index := wv.GetSelectedIndex(); index != expected {
		t.Errorf("Expected %d, got %d", expected, index)
	}
	
	// Test out of bounds - SetCurrentItem with invalid index won't actually change the current item
	// The list will keep the previous valid selection
	currentItem := wv.listView.GetCurrentItem()
	if currentItem >= len(wv.indices) {
		if index := wv.GetSelectedIndex(); index != -1 {
			t.Errorf("Expected -1 for out of bounds current item, got %d", index)
		}
	}
}

func TestWaterfallViewSetSelectedEntry(t *testing.T) {
	wv := NewWaterfallView()
	wv.indices = []int{5, 10, 15, 20}
	wv.listView.AddItem("Item 1", "", 0, nil)
	wv.listView.AddItem("Item 2", "", 0, nil)
	wv.listView.AddItem("Item 3", "", 0, nil)
	wv.listView.AddItem("Item 4", "", 0, nil)
	
	// Test setting to valid entry
	wv.SetSelectedEntry(15)
	if currentItem := wv.listView.GetCurrentItem(); currentItem != 2 {
		t.Errorf("Expected current item 2, got %d", currentItem)
	}
	
	// Test setting to non-existent entry
	originalItem := wv.listView.GetCurrentItem()
	wv.SetSelectedEntry(999)
	if currentItem := wv.listView.GetCurrentItem(); currentItem != originalItem {
		t.Errorf("Selection should not change for non-existent entry")
	}
}