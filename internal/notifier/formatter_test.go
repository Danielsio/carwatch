package notifier

import (
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/model"
)

func TestFormatListing(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token:        "abc123",
			Manufacturer: "Mazda",
			Model:        "3",
			Year:         2021,
			EngineVolume: 1998,
			GearBox:      "Automatic",
			Km:           85000,
			Hand:         2,
			City:         "Tel Aviv",
			Area:         "Center",
			Price:        95000,
			PageLink:     "https://www.yad2.co.il/item/abc123",
		},
	}

	msg := FormatListing(l)

	checks := []string{
		"Mazda 3",
		"2021",
		"2.0L",
		"Automatic",
		"85,000",
		"Tel Aviv",
		"95,000",
		"https://www.yad2.co.il/item/abc123",
	}

	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("message missing %q:\n%s", check, msg)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{100, "100"},
		{1000, "1,000"},
		{85000, "85,000"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatBatch_SingleListing(t *testing.T) {
	listings := []model.Listing{{
		RawListing: model.RawListing{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000},
	}}

	msg := FormatBatch(listings)
	if strings.Contains(msg, "2 New Listings") {
		t.Error("single listing should not use batch header")
	}
}

func TestFormatBatch_MultipleListing(t *testing.T) {
	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Mazda", Model: "3"}},
		{RawListing: model.RawListing{Token: "b", Manufacturer: "Mazda", Model: "3"}},
	}

	msg := FormatBatch(listings)
	if !strings.Contains(msg, "2 New Listings") {
		t.Error("batch should contain count header")
	}
	if !strings.Contains(msg, "[1/2]") || !strings.Contains(msg, "[2/2]") {
		t.Error("batch should contain numbered entries")
	}
}
