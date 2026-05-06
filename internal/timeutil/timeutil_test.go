package timeutil

import (
	"testing"
	"time"
)

func TestParseFlexible_ValidFormats(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect time.Time
	}{
		{
			name:   "datetime with fractional seconds",
			input:  "2024-03-15 10:30:45.123456789",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 123456789, time.UTC),
		},
		{
			name:   "datetime with timezone offset",
			input:  "2024-03-15 10:30:45.123-05:00",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 123000000, time.FixedZone("", -5*3600)),
		},
		{
			name:   "datetime without fractional seconds",
			input:  "2024-03-15 10:30:45",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:   "RFC3339Nano",
			input:  "2024-03-15T10:30:45.123456789Z",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 123456789, time.UTC),
		},
		{
			name:   "RFC3339",
			input:  "2024-03-15T10:30:45Z",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:   "RFC3339 with offset",
			input:  "2024-03-15T10:30:45+03:00",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 0, time.FixedZone("", 3*3600)),
		},
		{
			name:   "ISO datetime without timezone",
			input:  "2024-03-15T10:30:45",
			expect: time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFlexible(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.expect) {
				t.Errorf("got %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestParseFlexible_InvalidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"plain date no time", "2024-03-15"},
		{"garbage", "not-a-date"},
		{"US format", "03/15/2024"},
		{"unix timestamp", "1710500000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFlexible(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); len(got) == 0 {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestParseFlexible_FirstMatchWins(t *testing.T) {
	input := "2024-03-15 10:30:45.500000000"
	got, err := ParseFlexible(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Nanosecond() != 500000000 {
		t.Errorf("expected fractional seconds to be preserved, got ns=%d", got.Nanosecond())
	}
}
