package yad2

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

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

func TestParseFlexTime_Formats(t *testing.T) {
	// want, when set, asserts the exact instant — guarding against a regression
	// that parses a value in the wrong timezone (e.g. zone-less as UTC).
	tests := []struct {
		name string
		in   string
		zero bool
		want time.Time
	}{
		{"zoneless seconds (current feed)", "2025-02-09T10:31:37", false, time.Date(2025, 2, 9, 10, 31, 37, 0, israelTZ)},
		{"zoneless with millis", "2025-02-09T10:31:37.123", false, time.Date(2025, 2, 9, 10, 31, 37, 123_000_000, israelTZ)},
		{"rfc3339 Z", "2025-02-09T10:31:37Z", false, time.Date(2025, 2, 9, 10, 31, 37, 0, time.UTC)},
		{"rfc3339 offset", "2025-02-09T10:31:37+02:00", false, time.Date(2025, 2, 9, 8, 31, 37, 0, time.UTC)},
		{"rfc3339 Z with millis", "2025-02-09T10:31:37.123Z", false, time.Date(2025, 2, 9, 10, 31, 37, 123_000_000, time.UTC)},
		{"space separated", "2025-02-09 10:31:37", false, time.Date(2025, 2, 9, 10, 31, 37, 0, israelTZ)},
		{"date only", "2025-02-09", false, time.Date(2025, 2, 9, 0, 0, 0, 0, israelTZ)},
		{"surrounding whitespace", "  2025-02-09T10:31:37  ", false, time.Date(2025, 2, 9, 10, 31, 37, 0, israelTZ)},
		{"empty", "", true, time.Time{}},
		{"garbage", "not-a-date", true, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlexTime(tt.in)
			if got.IsZero() != tt.zero {
				t.Fatalf("parseFlexTime(%q) zero=%v, want zero=%v", tt.in, got.IsZero(), tt.zero)
			}
			if !tt.want.IsZero() && !got.Equal(tt.want) {
				t.Errorf("parseFlexTime(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseNextData_PostedAtUsesOriginalCreatedAt locks in the real Yad2 feed
// semantics: dates.createdAt is the per-listing original publish date, while
// dates.updatedAt / rebouncedAt are a single feed-wide "refreshed" timestamp.
// posted_at (CreatedAt) must track createdAt so a re-bumped month-old listing
// does not read as posted "today".
func TestParseNextData_PostedAtUsesOriginalCreatedAt(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"private":[{
				"token":"posted-at-1",
				"manufacturer":{"text":"Honda"},
				"model":{"text":"Civic"},
				"price":89000,
				"hand":2,
				"dates":{
					"createdAt":"2025-02-09T10:31:37",
					"updatedAt":"2025-02-14T21:30:57",
					"endsAt":"2025-03-26T00:00:00",
					"rebouncedAt":"2025-02-14T23:30:57"
				}
			}]
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
	if l.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set from dates.createdAt")
	}
	if want := parseFlexTime("2025-02-09T10:31:37"); !l.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want original createdAt %v", l.CreatedAt, want)
	}
	if l.CreatedAt.Equal(parseFlexTime("2025-02-14T21:30:57")) ||
		l.CreatedAt.Equal(parseFlexTime("2025-02-14T23:30:57")) {
		t.Error("CreatedAt must not equal the feed-wide updatedAt/rebouncedAt bump stamp")
	}
	if l.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set from dates.updatedAt")
	}
}

// TestParseNextData_UnparseableCreatedAtWarns covers the fallback path: an
// unrecognized dates.createdAt must not drop the listing, must leave CreatedAt
// unset (so the UI falls back to first-seen rather than a wrong date), and must
// emit a warning so a future Yad2 format change is visible instead of silent.
func TestParseNextData_UnparseableCreatedAtWarns(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"private":[{
				"token":"bad-date-1",
				"manufacturer":{"text":"Honda"},
				"model":{"text":"Civic"},
				"price":89000,
				"hand":2,
				"dates":{"createdAt":"29/05/2026"}
			}]
		}}}]}}}
	}`)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	listings, err := parseNextData(data, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected listing to survive unparseable date, got %d listings", len(listings))
	}
	if !listings[0].CreatedAt.IsZero() {
		t.Errorf("CreatedAt = %v, want zero for unparseable createdAt", listings[0].CreatedAt)
	}

	logged := buf.String()
	if !strings.Contains(logged, "unparseable listing created-at") {
		t.Errorf("expected warning about unparseable created-at, got logs: %q", logged)
	}
	if !strings.Contains(logged, "29/05/2026") {
		t.Errorf("expected warning to include the raw value, got logs: %q", logged)
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

func TestResolveImageURL_MetaDataCoverImage(t *testing.T) {
	item := feedItem{
		MetaData: struct {
			CoverImage  string `json:"coverImage"`
			CoverImg    string `json:"cover_image"`
			Description string `json:"description"`
		}{CoverImage: "https://img.yad2.co.il/primary.jpg"},
	}
	got := resolveImageURL(item)
	if got != "https://img.yad2.co.il/primary.jpg" {
		t.Errorf("resolveImageURL = %q, want primary coverImage", got)
	}
}

func TestResolveImageURL_MetaDataCoverImg(t *testing.T) {
	item := feedItem{
		MetaData: struct {
			CoverImage  string `json:"coverImage"`
			CoverImg    string `json:"cover_image"`
			Description string `json:"description"`
		}{CoverImg: "https://img.yad2.co.il/alt.jpg"},
	}
	got := resolveImageURL(item)
	if got != "https://img.yad2.co.il/alt.jpg" {
		t.Errorf("resolveImageURL = %q, want cover_image fallback", got)
	}
}

func TestResolveImageURL_TopLevelCoverImage(t *testing.T) {
	item := feedItem{
		CoverImageTop: "https://img.yad2.co.il/top.jpg",
	}
	got := resolveImageURL(item)
	if got != "https://img.yad2.co.il/top.jpg" {
		t.Errorf("resolveImageURL = %q, want top-level coverImage", got)
	}
}

func TestResolveImageURL_TopLevelCoverImgSnake(t *testing.T) {
	item := feedItem{
		CoverImgTop: "https://img.yad2.co.il/top_snake.jpg",
	}
	got := resolveImageURL(item)
	if got != "https://img.yad2.co.il/top_snake.jpg" {
		t.Errorf("resolveImageURL = %q, want top-level cover_image", got)
	}
}

func TestResolveImageURL_ImagesArray(t *testing.T) {
	item := feedItem{
		Images: []string{"https://img.yad2.co.il/first.jpg", "https://img.yad2.co.il/second.jpg"},
	}
	got := resolveImageURL(item)
	if got != "https://img.yad2.co.il/first.jpg" {
		t.Errorf("resolveImageURL = %q, want first from images array", got)
	}
}

func TestResolveImageURL_Empty(t *testing.T) {
	item := feedItem{}
	got := resolveImageURL(item)
	if got != "" {
		t.Errorf("resolveImageURL = %q, want empty", got)
	}
}

func TestParseNextData_ImageFallbackCoverImg(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "img-alt-1",
				 "manufacturer": {"text": "Toyota"},
				 "model": {"text": "Corolla"},
				 "price": 70000,
				 "hand": 1,
				 "metaData": {"cover_image": "https://img.yad2.co.il/snake.jpg", "description": "test"}}
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
	if listings[0].ImageURL != "https://img.yad2.co.il/snake.jpg" {
		t.Errorf("ImageURL = %q, want cover_image fallback", listings[0].ImageURL)
	}
}

func TestParseNextData_ImageFallbackTopLevel(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "img-top-1",
				 "manufacturer": {"text": "Toyota"},
				 "model": {"text": "Corolla"},
				 "price": 70000,
				 "hand": 1,
				 "coverImage": "https://img.yad2.co.il/top.jpg"}
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
	if listings[0].ImageURL != "https://img.yad2.co.il/top.jpg" {
		t.Errorf("ImageURL = %q, want top-level coverImage", listings[0].ImageURL)
	}
}

func TestParseNextData_ImageFallbackImagesArray(t *testing.T) {
	data := []byte(`{
		"props": {"pageProps": {"dehydratedState": {"queries": [{"state": {"data": {
			"private": [
				{"token": "img-arr-1",
				 "manufacturer": {"text": "Toyota"},
				 "model": {"text": "Corolla"},
				 "price": 70000,
				 "hand": 1,
				 "images": ["https://img.yad2.co.il/arr1.jpg", "https://img.yad2.co.il/arr2.jpg"]}
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
	if listings[0].ImageURL != "https://img.yad2.co.il/arr1.jpg" {
		t.Errorf("ImageURL = %q, want first from images array", listings[0].ImageURL)
	}
}

func TestParseNextData_ExtractsHPAndGearboxFromSubModel(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"hp-test",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"Premium אוט׳ 2.0 (165 כ״ס) [2017-2019]"},
				"vehicleDates":{"yearOfProduction":2018},
				"engineVolume":1998,
				"price":80000,
				"hand":{"id":2},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
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
	if l.HorsePower != 165 {
		t.Errorf("HorsePower = %d, want 165 (extracted from sub-model text)", l.HorsePower)
	}
	if l.GearBox != "אוטומט" {
		t.Errorf("GearBox = %q, want אוטומט (extracted from אוט׳ in sub-model text)", l.GearBox)
	}
}

func TestParseNextData_PreservesExplicitHPAndGearbox(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"explicit-test",
				"manufacturer":{"id":27},
				"model":{"id":10332},
				"subModel":{"text":"Premium אוט׳ 2.0 (165 כ״ס)"},
				"horsePower":200,
				"gearBox":{"text":"ידני"},
				"vehicleDates":{"yearOfProduction":2020},
				"price":90000,
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
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
	if l.HorsePower != 200 {
		t.Errorf("HorsePower = %d, want 200 (explicit field takes priority)", l.HorsePower)
	}
	if l.GearBox != "ידני" {
		t.Errorf("GearBox = %q, want ידני (explicit field takes priority)", l.GearBox)
	}
}

func TestParseNextData_ExtractsBodyTypeFromSubModel(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-test",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)"},
				"vehicleDates":{"yearOfProduction":2021},
				"engineVolume":1798,
				"price":95000,
				"hand":{"id":2},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "hatchback" {
		t.Errorf("BodyType = %q, want 'hatchback'", listings[0].BodyType)
	}
}

func TestParseNextData_BodyTypeFromDirectField(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-direct",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"LUXURY"},
				"bodyType":{"id":5,"text":"סדאן","english_text":"Sedan"},
				"vehicleDates":{"yearOfProduction":2021},
				"engineVolume":1998,
				"price":95000,
				"hand":{"id":2},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "sedan" {
		t.Errorf("BodyType = %q, want 'sedan' (from direct bodyType field)", listings[0].BodyType)
	}
}

func TestParseNextData_BodyTypeDirectFieldOverridesSubModel(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-override",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"Premium אוט׳ האצ'בק 2.0 (165 כ״ס)"},
				"bodyType":{"id":1,"text":"סדאן","english_text":"Sedan"},
				"vehicleDates":{"yearOfProduction":2021},
				"price":95000,
				"hand":{"id":2},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "sedan" {
		t.Errorf("BodyType = %q, want 'sedan' (direct field takes priority over subModel text)", listings[0].BodyType)
	}
}

func TestParseNextData_BodyTypeEmptyWhenNoMatch(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-empty",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"Premium אוט׳ 2.0 (165 כ״ס)"},
				"vehicleDates":{"yearOfProduction":2020},
				"price":90000,
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "" {
		t.Errorf("BodyType = %q, want empty string", listings[0].BodyType)
	}
}

func TestParseNextData_BodyTypeUnrecognizedDirectField(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-unknown",
				"manufacturer":{"id":27,"text":"מאזדה"},
				"model":{"id":10332,"text":"3"},
				"subModel":{"id":105280,"text":"LUXURY"},
				"bodyType":{"id":99,"text":"עגלה","english_text":"Cart"},
				"vehicleDates":{"yearOfProduction":2021},
				"price":95000,
				"hand":{"id":2},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "" {
		t.Errorf("BodyType = %q, want empty (unrecognized value falls through)", listings[0].BodyType)
	}
}

func TestParseNextData_BodyTypeFallsBackToHebrewText(t *testing.T) {
	data := []byte(`{
		"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
			"data":{"feed":{"feed_items":[{
				"token":"bt-hebrew-fallback",
				"manufacturer":{"id":53,"text":"טויוטה","english_text":"Toyota"},
				"model":{"id":10399,"text":"קורולה","english_text":"Corolla"},
				"subModel":{"id":105280,"text":"Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)","english_text":"Excite Plus"},
				"vehicleDates":{"yearOfProduction":2019},
				"engineVolume":1798,
				"price":87000,
				"hand":{"id":1},
				"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
			}]}}
		}}}]}}}
	}`)

	listings, err := parseNextData(data, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].BodyType != "hatchback" {
		t.Errorf("BodyType = %q, want 'hatchback' (from Hebrew text fallback when english_text lacks body type)", listings[0].BodyType)
	}
}

// TestParseNextData_BodyTypeYad2VocabularyOverSubModel covers the cases the old
// keyword matcher missed: Yad2's official "פנאי-שטח" (crossover/SUV), "ג'יפ שטח
// קשוח" and "ליפטבק" were absent from the keyword list, so they silently fell
// back to guessing from the sub-model. The structured Yad2 field now wins.
func TestParseNextData_BodyTypeYad2VocabularyOverSubModel(t *testing.T) {
	tests := []struct {
		name     string
		bodyType string // raw JSON for the bodyType field
		subModel string
		want     string
	}{
		{
			name:     "crossover leisure (id 7), sub-model has no body keyword",
			bodyType: `{"id":7,"text":"פנאי-שטח"}`,
			subModel: "Premium אוט׳ 2.0 (165 כ״ס)",
			want:     "suv",
		},
		{
			name:     "rugged jeep (id 6)",
			bodyType: `{"id":6,"text":"ג'יפ שטח קשוח"}`,
			subModel: "Limited 2.8 (204 כ״ס)",
			want:     "suv",
		},
		{
			name:     "liftback (id 14) classified, not left empty",
			bodyType: `{"id":14,"text":"ליפטבק"}`,
			subModel: "Sport אוט׳ 1.8",
			want:     "hatchback",
		},
		{
			name:     "Yad2 SUV wins over a sub-model that says האצ'בק",
			bodyType: `{"id":7,"text":"פנאי-שטח"}`,
			subModel: "Cross האצ'בק 1.5",
			want:     "suv",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`{
				"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
					"data":{"feed":{"feed_items":[{
						"token":"bt-vocab",
						"manufacturer":{"id":27,"text":"מאזדה"},
						"model":{"id":10332,"text":"3"},
						"subModel":{"id":105280,"text":"` + tt.subModel + `"},
						"bodyType":` + tt.bodyType + `,
						"vehicleDates":{"yearOfProduction":2021},
						"price":95000,
						"hand":{"id":2},
						"metaData":{"coverImage":"https://img.yad2.co.il/test.jpg"}
					}]}}
				}}}]}}}
			}`)
			listings, err := parseNextData(data, nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(listings) != 1 {
				t.Fatalf("expected 1 listing, got %d", len(listings))
			}
			if listings[0].BodyType != tt.want {
				t.Errorf("BodyType = %q, want %q", listings[0].BodyType, tt.want)
			}
		})
	}
}
