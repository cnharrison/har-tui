package har

import (
	"fmt"
	"time"
)

// Supported time formats for HAR datetime parsing, in order of preference
var supportedTimeFormats = []string{
	"2006-01-02T15:04:05.000Z",     // HAR standard format with milliseconds
	time.RFC3339Nano,                // RFC3339 with nanoseconds
	time.RFC3339,                    // RFC3339 standard
	"2006-01-02T15:04:05Z",         // HAR format without milliseconds
	"2006-01-02T15:04:05.000-07:00", // HAR with timezone offset
	"2006-01-02T15:04:05-07:00",     // RFC3339 with timezone offset
}

// ParseHARDateTime robustly parses HAR datetime strings with multiple format support
func ParseHARDateTime(dateTime string) (time.Time, error) {
	for _, format := range supportedTimeFormats {
		if t, err := time.Parse(format, dateTime); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse datetime: %s", dateTime)
}