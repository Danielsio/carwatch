package yad2

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
)

func TestParseListingsPage_ValidHTML(t *testing.T) {
	f, err := os.Open("../../../testdata/yad2_page.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	listings, err := ParseListingsPage(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}

	l := listings[0]
	if l.Token != "test-token-1" {
		t.Errorf("token = %q, want test-token-1", l.Token)
	}
	if l.Manufacturer != "Mazda" {
		t.Errorf("manufacturer = %q, want Mazda", l.Manufacturer)
	}
	if l.Model != "3" {
		t.Errorf("model = %q, want 3", l.Model)
	}
	if l.SubModel != "LUXURY" {
		t.Errorf("submodel = %q, want LUXURY", l.SubModel)
	}
	if l.Year != 2021 {
		t.Errorf("year = %d, want 2021", l.Year)
	}
	if l.Month != 6 {
		t.Errorf("month = %d, want 6", l.Month)
	}
	if l.EngineVolume != 1998 {
		t.Errorf("engine = %f, want 1998", l.EngineVolume)
	}
	if l.HorsePower != 165 {
		t.Errorf("hp = %d, want 165", l.HorsePower)
	}
	if l.GearBox != "Automatic" {
		t.Errorf("gearbox = %q, want Automatic", l.GearBox)
	}
	if l.Km != 85000 {
		t.Errorf("km = %d, want 85000", l.Km)
	}
	if l.Hand != 2 {
		t.Errorf("hand = %d, want 2", l.Hand)
	}
	if l.Price != 95000 {
		t.Errorf("price = %d, want 95000", l.Price)
	}
	if l.PageLink != "https://www.yad2.co.il/item/test-token-1" {
		t.Errorf("link = %q", l.PageLink)
	}
}

func TestParseListingsPage_Challenge(t *testing.T) {
	f, err := os.Open("../../../testdata/yad2_challenge.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	_, err = ParseListingsPage(f)
	if err == nil {
		t.Fatal("expected error for challenge page")
	}
	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Errorf("expected ErrChallenge, got: %v", err)
	}
}

func TestParseListingsPage_NoScript(t *testing.T) {
	html := `<html><body><p>No script tag here</p></body></html>`
	_, err := ParseListingsPage(strings.NewReader(html))
	if err == nil {
		t.Fatal("expected error for missing __NEXT_DATA__")
	}
	if !strings.Contains(err.Error(), "__NEXT_DATA__") {
		t.Errorf("error should mention __NEXT_DATA__, got: %v", err)
	}
}

func TestParseNextData_SkipsEmptyTokens(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/yad2_nextdata.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(listings) != 2 {
		t.Errorf("expected 2 listings (empty token skipped), got %d", len(listings))
	}

	for _, l := range listings {
		if l.Token == "" {
			t.Error("listing with empty token should have been skipped")
		}
	}
}

func TestParseNextData_FieldMapping(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/yad2_nextdata.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	first := listings[0]
	if first.Token != "abc123" {
		t.Errorf("token = %q", first.Token)
	}
	if first.Manufacturer != "Mazda" {
		t.Errorf("manufacturer = %q (english_text preferred)", first.Manufacturer)
	}
	if first.City != "תל אביב" {
		t.Errorf("city = %q (hebrew fallback)", first.City)
	}
	if first.Area != "מרכז" {
		t.Errorf("area = %q", first.Area)
	}
	if first.CreatedAt.IsZero() {
		t.Error("CreatedAt should be parsed")
	}
	if first.Description != "רכב במצב מעולה, יד שנייה, שמור מאוד" {
		t.Errorf("description = %q", first.Description)
	}

	second := listings[1]
	if second.Token != "def456" {
		t.Errorf("second token = %q", second.Token)
	}
	if second.EngineVolume != 1496 {
		t.Errorf("second engine = %f", second.EngineVolume)
	}
	if !second.UpdatedAt.IsZero() {
		t.Error("empty UpdatedAt should remain zero")
	}
}

func TestParseNextData_InvalidJSON(t *testing.T) {
	_, err := parseNextData([]byte(`{invalid`), nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseNextData_EmptyFeed(t *testing.T) {
	data := []byte(`{"props":{"pageProps":{"dehydratedState":{"queries":[]}}}}`)
	_, err := parseNextData(data, nil)
	if err == nil {
		t.Fatal("expected error for empty queries")
	}
}

func TestTextFromField_PrefersEnglish(t *testing.T) {
	tests := []struct {
		name string
		f    field
		want string
	}{
		{"english preferred", field{Text: "Hebrew", EnglishText: "English"}, "English"},
		{"hebrew fallback", field{Text: "Hebrew", EnglishText: ""}, "Hebrew"},
		{"both empty", field{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textFromField(tt.f)
			if got != tt.want {
				t.Errorf("textFromField() = %q, want %q", got, tt.want)
			}
		})
	}
}
