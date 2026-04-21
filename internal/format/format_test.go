package format

import "testing"

func TestNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{85000, "85,000"},
		{150000, "150,000"},
		{1500000, "1,500,000"},
		{-999, "-999"},
		{-1500, "-1,500"},
		{-1234567, "-1,234,567"},
	}
	for _, tt := range tests {
		got := Number(tt.input)
		if got != tt.want {
			t.Errorf("Number(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"plain text", "plain text"},
		{"under_score", "under\\_score"},
		{"*bold*", "\\*bold\\*"},
		{"[link](url)", "\\[link\\](url)"},
		{"`code`", "\\`code\\`"},
		{"a_b*c[d]e`f", "a\\_b\\*c\\[d\\]e\\`f"},
	}
	for _, tt := range tests {
		got := EscapeMarkdown(tt.input)
		if got != tt.want {
			t.Errorf("EscapeMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
