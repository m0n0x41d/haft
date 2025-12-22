package fpf

import (
	"testing"
	"time"
)

func TestComputeValidUntil(t *testing.T) {
	now := time.Now()

	tests := []struct {
		testType     string
		expectedDays int
	}{
		{"internal", 90},
		{"external", 60},
		{"unknown", 90}, // default
		{"", 90},        // empty defaults to 90
	}

	for _, tt := range tests {
		t.Run(tt.testType, func(t *testing.T) {
			result := computeValidUntil(tt.testType)
			parsed, err := time.Parse("2006-01-02", result)
			if err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			expected := now.AddDate(0, 0, tt.expectedDays)
			diff := parsed.Sub(expected.Truncate(24 * time.Hour))

			// Allow 1 day tolerance for edge cases around midnight
			if diff < -24*time.Hour || diff > 24*time.Hour {
				t.Errorf("testType=%q: got %s, expected ~%d days from now (%s)",
					tt.testType, result, tt.expectedDays, expected.Format("2006-01-02"))
			}
		})
	}
}
