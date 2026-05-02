package yad2

import (
	"encoding/json"
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
	defer func() { _ = f.Close() }()

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
	if l.PageLink != "https://www.yad2.co.il/vehicles/item/test-token-1" {
		t.Errorf("link = %q", l.PageLink)
	}
}

func TestParseListingsPage_Challenge(t *testing.T) {
	f, err := os.Open("../../../testdata/yad2_challenge.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

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

func TestParseNextData_FeedWithZeroItems(t *testing.T) {
	data := []byte(`{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"data":{"feed":{"feed_items":[]}}}}}]}}}}`)
	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("unexpected error for empty feed_items: %v", err)
	}
	if len(listings) != 0 {
		t.Errorf("expected 0 listings, got %d", len(listings))
	}
}

func TestParseNextData_FeedWithNullItems(t *testing.T) {
	data := []byte(`{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"data":{"feed":{"feed_items":null}}}}}]}}}}`)
	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("unexpected error for null feed_items: %v", err)
	}
	if listings == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(listings) != 0 {
		t.Errorf("expected 0 listings, got %d", len(listings))
	}
}

func TestParseHand(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want int
	}{
		{"nil", nil, 0},
		{"integer", json.RawMessage(`3`), 3},
		{"field object", json.RawMessage(`{"id":2,"text":"2"}`), 2},
		{"invalid json", json.RawMessage(`{invalid}`), 0},
		{"zero", json.RawMessage(`0`), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHand(tt.raw)
			if got != tt.want {
				t.Errorf("parseHand(%s) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTextFromField_PrefersEnglish(t *testing.T) {
	tests := []struct {
		name string
		f    field
		want string
	}{
		{"english_text preferred", field{Text: "Hebrew", EnglishText: "English"}, "English"},
		{"textEng fallback", field{Text: "Hebrew", TextEng: "English2"}, "English2"},
		{"english_text over textEng", field{Text: "Hebrew", EnglishText: "E1", TextEng: "E2"}, "E1"},
		{"hebrew fallback", field{Text: "Hebrew"}, "Hebrew"},
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

func TestParseNextData_CityEnglishTextFallback(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "eng-city", "manufacturer": {"text": "Honda"}, "model": {"text": "Civic"}, "price": 50000, "hand": 1,
				 "address": {"city": {"text": "", "english_text": "Tel Aviv", "id": 5000}, "area": {"text": "", "english_text": "Center", "id": 2}}}
			]
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].City != "Tel Aviv" {
		t.Errorf("city = %q, want 'Tel Aviv' (english_text fallback)", listings[0].City)
	}
	if listings[0].Area != "Center" {
		t.Errorf("area = %q, want 'Center' (english_text fallback)", listings[0].Area)
	}
}

func TestParseNextData_CityTextEngFallback(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "eng2-city", "manufacturer": {"text": "Honda"}, "model": {"text": "Civic"}, "price": 50000, "hand": 1,
				 "address": {"city": {"text": "", "textEng": "Haifa", "id": 4000}, "area": {"text": "", "textEng": "North", "id": 4}}}
			]
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].City != "Haifa" {
		t.Errorf("city = %q, want 'Haifa' (textEng fallback)", listings[0].City)
	}
	if listings[0].Area != "North" {
		t.Errorf("area = %q, want 'North' (textEng fallback)", listings[0].Area)
	}
}

func TestParseNextData_NewFeedFormat(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "new-fmt-1",
				 "manufacturer": {"id": 17, "text": "הונדה"},
				 "model": {"id": 10182, "text": "סיוויק"},
				 "subModel": {"id": 103617, "text": "Sport אוט׳ 1.8"},
				 "vehicleDates": {"yearOfProduction": 2010},
				 "engineType": {"id": 1101, "text": "בנזין"},
				 "engineVolume": 1799,
				 "hand": {"id": 2, "text": "יד שניה"},
				 "price": 16200,
				 "address": {"area": {"id": 91, "text": "אזור נצרת"}},
				 "metaData": {"coverImage": "https://img.yad2.co.il/test.jpg"}}
			]
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}

	l := listings[0]
	if l.Manufacturer != "הונדה" {
		t.Errorf("manufacturer = %q, want Hebrew fallback", l.Manufacturer)
	}
	if l.ManufacturerID != 17 {
		t.Errorf("manufacturer_id = %d, want 17", l.ManufacturerID)
	}
	if l.Year != 2010 {
		t.Errorf("year = %d, want 2010 (from vehicleDates)", l.Year)
	}
	if l.EngineVolume != 1799 {
		t.Errorf("engine = %f, want 1799 (from engineVolume)", l.EngineVolume)
	}
	if l.Km != 0 {
		t.Errorf("km = %d, want 0 (not in new feed format)", l.Km)
	}
	if l.Hand != 2 {
		t.Errorf("hand = %d, want 2 (from field object)", l.Hand)
	}
	if l.City != "" {
		t.Errorf("city = %q, want empty (not in feed)", l.City)
	}
	if l.Area != "אזור נצרת" {
		t.Errorf("area = %q, want Hebrew area text", l.Area)
	}
}

func TestParseNextData_DeduplicatesAcrossBuckets(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "dup1", "manufacturer": {"text": "Honda"}, "model": {"text": "Civic"}, "price": 50000, "hand": 1},
				{"token": "uniq", "manufacturer": {"text": "Honda"}, "model": {"text": "Civic"}, "price": 60000, "hand": 2}
			],
			"commercial": [
				{"token": "dup1", "manufacturer": {"text": "Honda"}, "model": {"text": "Civic"}, "price": 50000, "hand": 1}
			]
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 unique listings (dup removed), got %d", len(listings))
	}
	tokens := map[string]bool{}
	for _, l := range listings {
		if tokens[l.Token] {
			t.Errorf("duplicate token %q in results", l.Token)
		}
		tokens[l.Token] = true
	}
}
