package botcore

import "testing"

func TestNormalizeKeywords(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"", ""},
		{"  ", ""},
		{"hybrid", "hybrid"},
		{"hybrid, auto", "hybrid,auto"},
		{" hybrid , auto , ", "hybrid,auto"},
		{"hybrid,,auto", "hybrid,auto"},
	}
	for _, tt := range tests {
		got := NormalizeKeywords(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeKeywords(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToggleSource(t *testing.T) {
	tests := []struct {
		current, toggle, want string
	}{
		{"", "yad2", "yad2"},
		{"yad2", "winwin", "yad2,winwin"},
		{"yad2,winwin", "yad2", "winwin"},
		{"yad2,winwin", "winwin", "yad2"},
		{"winwin", "winwin", ""},
	}
	for _, tt := range tests {
		got := ToggleSource(tt.current, tt.toggle)
		if got != tt.want {
			t.Errorf("ToggleSource(%q, %q) = %q, want %q", tt.current, tt.toggle, got, tt.want)
		}
	}
}
