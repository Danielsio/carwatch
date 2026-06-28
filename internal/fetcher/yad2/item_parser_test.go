package yad2

import (
	"os"
	"strings"
	"testing"
)

func TestParseItemPage_ItemData(t *testing.T) {
	f, err := os.Open("../../../testdata/yad2_item_page.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	details, err := ParseItemPage(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if details.Km != 62000 {
		t.Errorf("km = %d, want 62000", details.Km)
	}
}

func TestParseItemPage_DehydratedState(t *testing.T) {
	f, err := os.Open("../../../testdata/yad2_item_dehydrated.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	details, err := ParseItemPage(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if details.Km != 120000 {
		t.Errorf("km = %d, want 120000", details.Km)
	}
}

func TestParseItemPage_Challenge(t *testing.T) {
	html := `<html><body>Are you for real</body></html>`
	_, err := ParseItemPage(strings.NewReader(html))
	if err == nil {
		t.Fatal("expected error for challenge page")
	}
	if !strings.Contains(err.Error(), "challenge") {
		t.Errorf("error should mention challenge, got: %v", err)
	}
}

func TestParseItemPage_NoScript(t *testing.T) {
	html := `<html><body><p>No script here</p></body></html>`
	_, err := ParseItemPage(strings.NewReader(html))
	if err == nil {
		t.Fatal("expected error for missing __NEXT_DATA__")
	}
}

func TestParseItemPage_WithAddress(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 237000,
  "coverImage": "https://img.yad2.co.il/test.jpg",
  "address": {
    "city": {"id": "1724", "text": "נצרת", "textEng": "nazareth"},
    "area": {"id": 91, "text": "אזור נצרת", "textEng": "nazareth_area"}
  }
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.Km != 237000 {
		t.Errorf("km = %d, want 237000", details.Km)
	}
	if details.City != "nazareth" {
		t.Errorf("city = %q, want nazareth (textEng preferred)", details.City)
	}
	if details.Area != "nazareth_area" {
		t.Errorf("area = %q, want nazareth_area", details.Area)
	}
}

func TestParseItemPage_AddressHebrewFallback(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 100000,
  "address": {
    "city": {"id": "5000", "text": "תל אביב"},
    "area": {"id": 2, "text": "מרכז"}
  }
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.City != "תל אביב" {
		t.Errorf("city = %q, want Hebrew fallback", details.City)
	}
	if details.Area != "מרכז" {
		t.Errorf("area = %q, want Hebrew fallback", details.Area)
	}
}

func TestParseItemPage_NoKm(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"token":"abc","km":0}}}}
</script>
</body></html>`
	_, err := ParseItemPage(strings.NewReader(html))
	if err == nil {
		t.Fatal("expected error when km is 0")
	}
}

func TestParseItemPage_CoverImgSnakeCase(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 50000,
  "cover_image": "https://img.yad2.co.il/snake.jpg"
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.ImageURL != "https://img.yad2.co.il/snake.jpg" {
		t.Errorf("ImageURL = %q, want cover_image fallback", details.ImageURL)
	}
}

func TestNormalizeOwnership(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Hebrew inputs
		{"פרטי", "private"},
		{"בעלות פרטי", "private"},
		{"ליסינג", "lease"},
		{"השכרה", "rental"},
		// English inputs
		{"private", "private"},
		{"Private", "private"},
		{"PRIVATE", "private"},
		{"lease", "lease"},
		{"Lease", "lease"},
		{"leasing", "lease"},
		{"Leasing", "lease"},
		{"rental", "rental"},
		{"Rental", "rental"},
		// English substring matches in Hebrew context
		{"leasing company", "lease"},
		{"rental car", "rental"},
		// Unknown / empty
		{"", ""},
		{"  ", ""},
		{"unknown", ""},
		{"other", ""},
		// Whitespace trimming
		{"  private  ", "private"},
		{"  ליסינג  ", "lease"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeOwnership(tt.input)
			if got != tt.want {
				t.Errorf("normalizeOwnership(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseItemPage_Ownership(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 50000,
  "originalOwnership": {"text": "ליסינג", "textEng": "leasing", "id": 2},
  "currentOwnership": {"text": "פרטי", "textEng": "private", "id": 1}
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.OriginalOwnership != "lease" {
		t.Errorf("OriginalOwnership = %q, want lease", details.OriginalOwnership)
	}
	if details.CurrentOwnership != "private" {
		t.Errorf("CurrentOwnership = %q, want private", details.CurrentOwnership)
	}
}

func TestParseItemPage_OwnershipFallbackToPrevious(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 60000,
  "previousOwnership": {"text": "השכרה", "textEng": "rental", "id": 3}
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.OriginalOwnership != "rental" {
		t.Errorf("OriginalOwnership = %q, want rental (fallback from previousOwnership)", details.OriginalOwnership)
	}
}

func TestParseItemPage_OwnershipFallbackToOwnership(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 70000,
  "ownership": {"text": "", "textEng": "private", "id": 1}
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.OriginalOwnership != "private" {
		t.Errorf("OriginalOwnership = %q, want private (fallback from ownership)", details.OriginalOwnership)
	}
}

func TestParseItemPage_BodyType(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 33000,
  "bodyType": {"text": "האצ'בק", "textEng": "Hatchback", "id": 5}
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.BodyType != "hatchback" {
		t.Errorf("BodyType = %q, want hatchback", details.BodyType)
	}
}

func TestParseItemPage_BodyTypeHebrewOnly(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 50000,
  "bodyType": {"text": "סדאן", "id": 1}
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.BodyType != "sedan" {
		t.Errorf("BodyType = %q, want sedan", details.BodyType)
	}
}

func TestParseItemPage_BodyTypeAbsent(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 80000
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.BodyType != "" {
		t.Errorf("BodyType = %q, want empty when field absent", details.BodyType)
	}
}

func TestParseItemPage_ImagesArrayFallback(t *testing.T) {
	html := `<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{
  "km": 75000,
  "images": ["https://img.yad2.co.il/arr1.jpg", "https://img.yad2.co.il/arr2.jpg"]
}}}]}}}}
</script>
</body></html>`
	details, err := ParseItemPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if details.ImageURL != "https://img.yad2.co.il/arr1.jpg" {
		t.Errorf("ImageURL = %q, want first from images array", details.ImageURL)
	}
}
