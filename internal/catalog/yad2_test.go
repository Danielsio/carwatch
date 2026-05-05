package catalog

import (
	"context"
	"fmt"
	"testing"
)

func fakeNextDataHTML(listings string) []byte {
	return []byte(fmt.Sprintf(`<html><head>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"private":[%s]}}}]}}}}
</script></head><body></body></html>`, listings))
}

func TestParseCatalogFromHTML_ExtractsManufacturersAndModels(t *testing.T) {
	html := fakeNextDataHTML(`
		{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10222,"text":"קאמרי","english_text":"Camry"}},
		{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10226,"text":"קורולה","english_text":"Corolla"}},
		{"manufacturer":{"id":27,"text":"מאזדה","english_text":"Mazda"},"model":{"id":10332,"text":"3","english_text":"3"}}
	`)

	result, err := ParseCatalogFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Manufacturers) != 2 {
		t.Fatalf("expected 2 manufacturers, got %d", len(result.Manufacturers))
	}

	toyota := result.Manufacturers[19]
	if toyota.Name != "Toyota" || toyota.NameHe != "טויוטה" {
		t.Errorf("Toyota = %+v", toyota)
	}

	mazda := result.Manufacturers[27]
	if mazda.Name != "Mazda" || mazda.NameHe != "מאזדה" {
		t.Errorf("Mazda = %+v", mazda)
	}

	toyotaModels := result.Models[19]
	if len(toyotaModels) != 2 {
		t.Fatalf("expected 2 Toyota models, got %d", len(toyotaModels))
	}
	if toyotaModels[10222].Name != "Camry" || toyotaModels[10222].NameHe != "קאמרי" {
		t.Errorf("Camry = %+v", toyotaModels[10222])
	}
	if toyotaModels[10226].Name != "Corolla" {
		t.Errorf("Corolla = %+v", toyotaModels[10226])
	}
}

func TestParseCatalogFromHTML_BotProtection(t *testing.T) {
	html := []byte(`<html><body>Are you for real</body></html>`)
	_, err := ParseCatalogFromHTML(html)
	if err == nil {
		t.Fatal("expected error for bot protection page")
	}
}

func TestParseCatalogFromHTML_NoNextData(t *testing.T) {
	html := []byte(`<html><body>no data here</body></html>`)
	_, err := ParseCatalogFromHTML(html)
	if err == nil {
		t.Fatal("expected error when __NEXT_DATA__ missing")
	}
}

func TestParseCatalogFromHTML_EmptyListings(t *testing.T) {
	html := fakeNextDataHTML("")
	_, err := ParseCatalogFromHTML(html)
	if err == nil {
		t.Fatal("expected error for empty listings")
	}
}

func TestParseCatalogFromHTML_TextEngFallback(t *testing.T) {
	html := fakeNextDataHTML(`
		{"manufacturer":{"id":5,"text":"הונדה","textEng":"Honda"},"model":{"id":100,"text":"סיוויק","textEng":"Civic"}}
	`)

	result, err := ParseCatalogFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	honda := result.Manufacturers[5]
	if honda.Name != "Honda" {
		t.Errorf("expected Honda from textEng fallback, got %q", honda.Name)
	}
	if honda.NameHe != "הונדה" {
		t.Errorf("expected הונדה, got %q", honda.NameHe)
	}
}

func TestParseCatalogFromHTML_DuplicateListingsDeduped(t *testing.T) {
	html := fakeNextDataHTML(`
		{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10222,"text":"קאמרי","english_text":"Camry"}},
		{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10222,"text":"קאמרי","english_text":"Camry"}}
	`)

	result, err := ParseCatalogFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Manufacturers) != 1 {
		t.Errorf("expected 1 manufacturer (deduped), got %d", len(result.Manufacturers))
	}
	if len(result.Models[19]) != 1 {
		t.Errorf("expected 1 model (deduped), got %d", len(result.Models[19]))
	}
}

func TestParseCatalogFromHTML_LegacyFormat(t *testing.T) {
	html := []byte(`<html><head>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"data":{"feed":{"feed_items":[
{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10222,"text":"קאמרי","english_text":"Camry"}}
]}}}}}]}}}}
</script></head><body></body></html>`)

	result, err := ParseCatalogFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Manufacturers) != 1 {
		t.Errorf("expected 1 manufacturer from legacy format, got %d", len(result.Manufacturers))
	}
}

func TestFetchCatalogFromYad2_FetcherError(t *testing.T) {
	fetcher := &StaticPageFetcher{Err: fmt.Errorf("network down")}
	_, err := FetchCatalogFromYad2(context.Background(), fetcher)
	if err == nil {
		t.Fatal("expected error when fetcher fails")
	}
}

func TestFetchCatalogFromYad2_Success(t *testing.T) {
	html := fakeNextDataHTML(`
		{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10222,"text":"קאמרי","english_text":"Camry"}}
	`)
	fetcher := &StaticPageFetcher{HTML: html}

	result, err := FetchCatalogFromYad2(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Manufacturers) != 1 {
		t.Errorf("expected 1 manufacturer, got %d", len(result.Manufacturers))
	}
}

func TestDynamicCatalog_Load_WithYad2Fetcher(t *testing.T) {
	html := fakeNextDataHTML(`
		{"manufacturer":{"id":900,"text":"טסט","english_text":"TestMfr"},"model":{"id":9001,"text":"דגם","english_text":"TestModel"}}
	`)
	fetcher := &StaticPageFetcher{HTML: html}

	d := NewDynamic(testLogger)
	d.Load(context.Background(), fetcher)

	if name := d.ManufacturerName(900); name != "TestMfr" {
		t.Errorf("expected TestMfr from yad2 fetch, got %q", name)
	}
	if name := d.ModelName(900, 9001); name != "TestModel" {
		t.Errorf("expected TestModel from yad2 fetch, got %q", name)
	}

	// Static fallback entries should still be present
	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected static fallback manufacturers merged, got %d", len(mfrs))
	}
}

func TestDynamicCatalog_Load_FetchFailsFallsBackToStatic(t *testing.T) {
	fetcher := &StaticPageFetcher{Err: fmt.Errorf("bot protection")}

	d := NewDynamic(testLogger)
	d.Load(context.Background(), fetcher)

	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected static fallback on fetch failure, got %d manufacturers", len(mfrs))
	}
}

func TestDynamicCatalog_Load_Yad2OverridesStaticButKeepsMissing(t *testing.T) {
	// Yad2 provides Toyota with Hebrew name; static also has Toyota.
	// After load, Toyota should use the Yad2 data but static-only entries remain.
	html := fakeNextDataHTML(`
		{"manufacturer":{"id":19,"text":"טויוטה","english_text":"Toyota"},"model":{"id":10222,"text":"קאמרי","english_text":"Camry"}}
	`)
	fetcher := &StaticPageFetcher{HTML: html}

	d := NewDynamic(testLogger)
	d.Load(context.Background(), fetcher)

	// Toyota from yad2
	if name := d.ManufacturerName(19); name != "Toyota" {
		t.Errorf("Toyota name = %q", name)
	}

	// Camry model from yad2
	if name := d.ModelName(19, 10222); name != "Camry" {
		t.Errorf("Camry model = %q", name)
	}

	// Static-only manufacturer (e.g., Hyundai ID=21) should still exist
	if name := d.ManufacturerName(21); name == "Unknown" {
		t.Error("static-only manufacturer Hyundai should be preserved")
	}
}
