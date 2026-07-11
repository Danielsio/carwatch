package timeutil

import (
	"testing"
	"time"
)

func TestParseFlexTime_Formats(t *testing.T) {
	// want, when set, asserts the exact instant — guarding against a regression
	// that parses a value in the wrong timezone (e.g. zone-less as UTC).
	tests := []struct {
		name string
		in   string
		zero bool
		want time.Time
	}{
		{"zoneless seconds (current feed)", "2025-02-09T10:31:37", false, time.Date(2025, 2, 9, 10, 31, 37, 0, IsraelTZ)},
		{"zoneless with millis", "2025-02-09T10:31:37.123", false, time.Date(2025, 2, 9, 10, 31, 37, 123_000_000, IsraelTZ)},
		{"rfc3339 Z", "2025-02-09T10:31:37Z", false, time.Date(2025, 2, 9, 10, 31, 37, 0, time.UTC)},
		{"rfc3339 offset", "2025-02-09T10:31:37+02:00", false, time.Date(2025, 2, 9, 8, 31, 37, 0, time.UTC)},
		{"rfc3339 Z with millis", "2025-02-09T10:31:37.123Z", false, time.Date(2025, 2, 9, 10, 31, 37, 123_000_000, time.UTC)},
		{"space separated", "2025-02-09 10:31:37", false, time.Date(2025, 2, 9, 10, 31, 37, 0, IsraelTZ)},
		{"space separated with nanos", "2025-02-09 10:31:37.5", false, time.Date(2025, 2, 9, 10, 31, 37, 500_000_000, IsraelTZ)},
		{"date only", "2025-02-09", false, time.Date(2025, 2, 9, 0, 0, 0, 0, IsraelTZ)},
		{"surrounding whitespace", "  2025-02-09T10:31:37  ", false, time.Date(2025, 2, 9, 10, 31, 37, 0, IsraelTZ)},
		{"empty", "", true, time.Time{}},
		{"garbage", "not-a-date", true, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFlexTime(tt.in)
			if got.IsZero() != tt.zero {
				t.Fatalf("ParseFlexTime(%q) zero=%v, want zero=%v", tt.in, got.IsZero(), tt.zero)
			}
			if !tt.want.IsZero() && !got.Equal(tt.want) {
				t.Errorf("ParseFlexTime(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseFlexTime_ZonelessIsNotUTC is the regression guard for the bug this
// package exists to prevent: a zone-less feed timestamp read as UTC lands
// ~2-3h early. Israel is UTC+2 (winter) / UTC+3 (summer), never UTC.
func TestParseFlexTime_ZonelessIsNotUTC(t *testing.T) {
	got := ParseFlexTime("2025-02-09T10:31:37")
	if asUTC := time.Date(2025, 2, 9, 10, 31, 37, 0, time.UTC); got.Equal(asUTC) {
		t.Errorf("zone-less timestamp parsed as UTC (%v) — must be Israel local time", got)
	}
	if _, offset := got.Zone(); offset == 0 {
		t.Errorf("zone-less timestamp has zero UTC offset, want Israel local offset")
	}
}
