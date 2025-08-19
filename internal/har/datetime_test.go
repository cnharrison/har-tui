package har

import (
	"testing"
	"time"
)

func TestParseHARDateTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		expected string
	}{
		{
			name:     "HAR standard format with milliseconds",
			input:    "2023-01-15T14:30:45.123Z",
			wantErr:  false,
			expected: "2023-01-15T14:30:45.123Z",
		},
		{
			name:     "HAR format without milliseconds",
			input:    "2023-01-15T14:30:45Z",
			wantErr:  false,
			expected: "2023-01-15T14:30:45Z",
		},
		{
			name:     "RFC3339 format",
			input:    "2023-01-15T14:30:45-08:00",
			wantErr:  false,
			expected: "2023-01-15T22:30:45Z", // UTC conversion
		},
		{
			name:     "HAR with timezone offset",
			input:    "2023-01-15T14:30:45.123-08:00",
			wantErr:  false,
			expected: "2023-01-15T22:30:45.123Z", // UTC conversion
		},
		{
			name:     "RFC3339Nano format",
			input:    "2023-01-15T14:30:45.123456789Z",
			wantErr:  false,
			expected: "2023-01-15T14:30:45.123456789Z",
		},
		{
			name:    "Invalid format",
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "Empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseHARDateTime(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseHARDateTime() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseHARDateTime() unexpected error: %v", err)
				return
			}
			
			// Convert result to UTC and format for comparison
			resultUTC := result.UTC().Format(time.RFC3339Nano)
			expectedTime, _ := time.Parse(time.RFC3339Nano, tt.expected)
			expectedUTC := expectedTime.UTC().Format(time.RFC3339Nano)
			
			if resultUTC != expectedUTC {
				t.Errorf("ParseHARDateTime() = %v, want %v", resultUTC, expectedUTC)
			}
		})
	}
}

func BenchmarkParseHARDateTime(b *testing.B) {
	testInput := "2023-01-15T14:30:45.123Z"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseHARDateTime(testInput)
		if err != nil {
			b.Fatal(err)
		}
	}
}