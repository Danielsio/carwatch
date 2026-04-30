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
