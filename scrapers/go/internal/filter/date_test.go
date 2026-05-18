package filter

import (
	"testing"
	"time"
)

func TestIsRecentJob(t *testing.T) {
	now := time.Now()
	recentISO := now.Add(-5 * 24 * time.Hour).Format("2006-01-02")
	oldISO := now.Add(-65 * 24 * time.Hour).Format("2006-01-02")

	recentSlash := now.Add(-5 * 24 * time.Hour).Format("02/01/2006")
	oldSlash := now.Add(-65 * 24 * time.Hour).Format("02/01/2006")

	tests := []struct {
		name     string
		dateStr  string
		expected bool
	}{
		{
			name:     "Empty date",
			dateStr:  "",
			expected: true,
		},
		{
			name:     "N/A date",
			dateStr:  "N/A",
			expected: true,
		},
		{
			name:     "Recent keyword",
			dateStr:  "Recent",
			expected: true,
		},
		{
			name:     "Recent ISO format",
			dateStr:  recentISO,
			expected: true,
		},
		{
			name:     "Old ISO format",
			dateStr:  oldISO,
			expected: false,
		},
		{
			name:     "Recent slash format",
			dateStr:  recentSlash,
			expected: true,
		},
		{
			name:     "Old slash format",
			dateStr:  oldSlash,
			expected: false,
		},
		{
			name:     "Current year fallback",
			dateStr:  "Year 2026",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRecentJob(tt.dateStr)
			if got != tt.expected {
				t.Errorf("IsRecentJob(%q) = %v; want %v", tt.dateStr, got, tt.expected)
			}
		})
	}
}
