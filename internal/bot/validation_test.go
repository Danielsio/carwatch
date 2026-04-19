package bot

import (
	"strconv"
	"testing"
)

func TestYearValidation(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"2020", true},
		{"1990", true},
		{"2030", true},
		{"1989", false},
		{"2031", false},
		{"abc", false},
		{"", false},
	}
	for _, tt := range tests {
		year, err := strconv.Atoi(tt.input)
		valid := err == nil && year >= 1990 && year <= 2030
		if valid != tt.valid {
			t.Errorf("year %q: valid=%v, want %v", tt.input, valid, tt.valid)
		}
	}
}

func TestPriceValidation(t *testing.T) {
	tests := []struct {
		input string
		valid bool
		value int
	}{
		{"150000", true, 150000},
		{"150,000", true, 150000},
		{"1000", true, 1000},
		{"999", false, 0},
		{"10000001", false, 0},
		{"abc", false, 0},
	}
	for _, tt := range tests {
		cleaned := ""
		for _, c := range tt.input {
			if c != ',' {
				cleaned += string(c)
			}
		}
		price, err := strconv.Atoi(cleaned)
		valid := err == nil && price >= 1000 && price <= 10000000
		if valid != tt.valid {
			t.Errorf("price %q: valid=%v, want %v", tt.input, valid, tt.valid)
		}
		if valid && price != tt.value {
			t.Errorf("price %q: value=%d, want %d", tt.input, price, tt.value)
		}
	}
}

func TestCallbackParsing(t *testing.T) {
	tests := []struct {
		data   string
		prefix string
		id     int
	}{
		{"mfr:27", cbPrefixMfr, 27},
		{"mdl:10332", cbPrefixModel, 10332},
		{"eng:2000", cbPrefixEngine, 2000},
		{"eng:0", cbPrefixEngine, 0},
	}
	for _, tt := range tests {
		trimmed := tt.data[len(tt.prefix):]
		id, err := strconv.Atoi(trimmed)
		if err != nil {
			t.Errorf("parse %q after removing %q: %v", tt.data, tt.prefix, err)
			continue
		}
		if id != tt.id {
			t.Errorf("parse %q: got %d, want %d", tt.data, id, tt.id)
		}
	}
}

func TestSearchNameGeneration(t *testing.T) {
	tests := []struct {
		manufacturer string
		model        string
		want         string
	}{
		{"Mazda", "3", "mazda-3"},
		{"Toyota", "Corolla", "toyota-corolla"},
		{"BMW", "3 Series", "bmw-3 series"},
	}
	for _, tt := range tests {
		got := toLowerCase(tt.manufacturer) + "-" + toLowerCase(tt.model)
		if got != tt.want {
			t.Errorf("name(%q, %q) = %q, want %q", tt.manufacturer, tt.model, got, tt.want)
		}
	}
}

func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}
