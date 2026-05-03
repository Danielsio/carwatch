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
