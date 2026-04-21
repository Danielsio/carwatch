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
	}
	for _, tt := range tests {
		got := Number(tt.input)
		if got != tt.want {
			t.Errorf("Number(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
