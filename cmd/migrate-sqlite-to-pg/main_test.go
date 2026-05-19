package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPlaceholders(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "$1"},
		{2, "$1, $2"},
		{3, "$1, $2, $3"},
		{5, "$1, $2, $3, $4, $5"},
	}

	for _, tt := range tests {
		got := buildPlaceholders(tt.n)
		if got != tt.want {
			t.Errorf("buildPlaceholders(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestConvertValue_TimeStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		year  int
		month time.Month
		day   int
	}{
		{"RFC3339Nano", "2024-06-15T10:30:45.123456789Z", 2024, time.June, 15},
		{"RFC3339", "2024-06-15T10:30:45+03:00", 2024, time.June, 15},
		{"datetime with tz offset", "2024-06-15 10:30:45.123456-05:00", 2024, time.June, 15},
		{"datetime microseconds", "2024-06-15 10:30:45.123456", 2024, time.June, 15},
		{"datetime no frac", "2024-06-15 10:30:45", 2024, time.June, 15},
		{"ISO with Z", "2024-06-15T10:30:45Z", 2024, time.June, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertValue(tt.input)
			tm, ok := got.(time.Time)
			if !ok {
				t.Fatalf("expected time.Time, got %T: %v", got, got)
			}
			if tm.Location() != time.UTC {
				t.Errorf("expected UTC, got %v", tm.Location())
			}
			if tm.Year() != tt.year || tm.Month() != tt.month || tm.Day() != tt.day {
				t.Errorf("got %v, expected date %d-%02d-%02d", tm, tt.year, tt.month, tt.day)
			}
		})
	}
}

func TestConvertValue_NonTimeStrings(t *testing.T) {
	tests := []string{
		"hello",
		"2024-06-15",
		"not a date",
		"",
		"12345",
	}

	for _, input := range tests {
		got := convertValue(input)
		s, ok := got.(string)
		if !ok {
			t.Errorf("convertValue(%q) returned %T, want string", input, got)
			continue
		}
		if s != input {
			t.Errorf("convertValue(%q) = %q, want same string back", input, s)
		}
	}
}

func TestConvertValue_ByteSlice(t *testing.T) {
	input := []byte("hello world")
	got := convertValue(input)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "hello world" {
		t.Errorf("got %q, want %q", s, "hello world")
	}
}

func TestConvertValue_OtherTypes(t *testing.T) {
	tests := []any{
		int64(42),
		float64(3.14),
		true,
		nil,
	}

	for _, input := range tests {
		got := convertValue(input)
		if got != input {
			t.Errorf("convertValue(%v) = %v, want same value", input, got)
		}
	}
}

func TestTableSpec_ColumnCountMatchesList(t *testing.T) {
	tables := []tableSpec{
		{name: "users", columns: "chat_id, username, state, state_data, created_at, active, digest_mode, digest_interval, digest_last_flushed, language, tier, tier_expires_at, trial_used, daily_digest, daily_digest_time, daily_digest_last_sent, channel, channel_id, linked_web_id, last_seen_at", count: 20},
		{name: "searches", columns: "id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_min, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, seller_filter, gear_box, price_only, photo_only, active, created_at, share_token", count: 23},
		{name: "seen_listings", columns: "token, chat_id, search_id, first_seen_at", count: 4},
		{name: "listing_user_seen", columns: "chat_id, token, seen_at", count: 3},
		{name: "pending_notifications", columns: "id, recipient, search_name, payload, created_at", count: 5},
		{name: "price_history", columns: "id, token, price, observed_at", count: 4},
		{name: "listing_history", columns: "token, chat_id, search_id, search_name, manufacturer, model, sub_model, sub_model_id, year, price, km, hand, city, page_link, image_url, engine_volume, horse_power, engine_type, gear_box, description, is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, removed_at, first_seen_at", count: 28},
		{name: "pending_digest", columns: "id, chat_id, listing_payload, created_at", count: 4},
		{name: "saved_listings", columns: "chat_id, token, saved_at", count: 3},
		{name: "hidden_listings", columns: "chat_id, token, hidden_at", count: 3},
		{name: "link_tokens", columns: "token, web_chat_id, created_at, expires_at, used", count: 5},
	}

	for _, tt := range tables {
		cols := strings.Split(tt.columns, ", ")
		if len(cols) != tt.count {
			t.Errorf("table %s: column list has %d entries but count is %d\ncolumns: %v",
				tt.name, len(cols), tt.count, cols)
		}
	}
}
