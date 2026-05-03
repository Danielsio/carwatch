package yad2

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

func newTestEnricher(t *testing.T, handler http.Handler, cfg EnricherConfig) *Enricher {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := newPlainClient([]string{"test-ua"}, "")
	if err != nil {
		t.Fatalf("create plain client: %v", err)
	}

	fetcher := &Yad2Fetcher{
		client:     client,
		baseURL:    srv.URL,
		logger:     slog.Default(),
		userAgents: []string{"test-ua"},
	}

	return NewEnricher(fetcher, slog.Default(), cfg)
}

func itemPageHandler(km int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":%d,"address":{"city":{"text":"תל אביב","textEng":"tel_aviv"},"area":{"text":"מרכז","textEng":"center"}}}}}}
</script></html>`, km)
	}
}

func TestEnricher_FillsMissingKm(t *testing.T) {
	enricher := newTestEnricher(t, itemPageHandler(75000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 50000, City: "Haifa"},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 2 {
		t.Errorf("enriched = %d, want 2", count)
	}
	if listings[0].Km != 50000 {
		t.Errorf("listing[0].Km = %d, want 50000 (unchanged)", listings[0].Km)
	}
	if listings[1].Km != 75000 {
		t.Errorf("listing[1].Km = %d, want 75000", listings[1].Km)
	}
	if listings[2].Km != 75000 {
		t.Errorf("listing[2].Km = %d, want 75000", listings[2].Km)
	}
}

func TestEnricher_RespectsMaxPerCycle(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":10000}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 1,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1 (max per cycle)", count)
	}
	if listings[0].Km != 10000 {
		t.Errorf("listing[0].Km = %d, want 10000", listings[0].Km)
	}
	if listings[1].Km != 0 {
		t.Errorf("listing[1].Km = %d, want 0 (not enriched)", listings[1].Km)
	}
	if got := requestCount.Load(); got != 1 {
		t.Errorf("requests = %d, want 1 (budget limits attempts)", got)
	}
}

func TestEnricher_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	enricher := newTestEnricher(t, itemPageHandler(10000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
	}

	count := enricher.Enrich(ctx, listings)
	if count > 0 {
		t.Errorf("enriched = %d, want 0 (ctx canceled before first attempt)", count)
	}
}

func TestEnricher_SkipsOnError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (error)", count)
	}
	if listings[0].Km != 0 {
		t.Errorf("listing[0].Km = %d, want 0 (unchanged on error)", listings[0].Km)
	}
}

func TestEnricher_AllHaveKm(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":50000}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 10000, ImageURL: "https://img.yad2.co.il/a.jpg", City: "Tel Aviv"},
		{Token: "b", Km: 20000, ImageURL: "https://img.yad2.co.il/b.jpg", City: "Haifa"},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all have km, image, and city)", count)
	}
	if got := requestCount.Load(); got != 0 {
		t.Errorf("requests = %d, want 0 (no fetches needed)", got)
	}
}

func TestEnricher_FillsMissingImageOnly(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":50000,"coverImage":"https://img.yad2.co.il/test.jpg","address":{"city":{"text":"חיפה","textEng":"haifa"}}}}}}
</script></html>`)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 50000, ImageURL: "", City: "Haifa"},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1 (image only)", count)
	}
	if listings[0].ImageURL != "https://img.yad2.co.il/test.jpg" {
		t.Errorf("listing[0].ImageURL = %q, want test.jpg URL", listings[0].ImageURL)
	}
	if listings[0].Km != 50000 {
		t.Errorf("listing[0].Km = %d, want 50000 (unchanged)", listings[0].Km)
	}
}

func TestEnricher_FillsMissingCity(t *testing.T) {
	enricher := newTestEnricher(t, itemPageHandler(75000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 50000, ImageURL: "https://img.yad2.co.il/a.jpg", City: ""},
		{Token: "b", Km: 50000, ImageURL: "https://img.yad2.co.il/b.jpg", City: "Haifa"},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1 (only first needs city)", count)
	}
	if listings[0].City != "tel_aviv" {
		t.Errorf("listing[0].City = %q, want tel_aviv", listings[0].City)
	}
	if listings[1].City != "Haifa" {
		t.Errorf("listing[1].City = %q, want Haifa (unchanged)", listings[1].City)
	}
}

func TestEnricher_FillsKmAndCity(t *testing.T) {
	enricher := newTestEnricher(t, itemPageHandler(90000), EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 10,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0, City: ""},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 1 {
		t.Errorf("enriched = %d, want 1", count)
	}
	if listings[0].Km != 90000 {
		t.Errorf("listing[0].Km = %d, want 90000", listings[0].Km)
	}
	if listings[0].City != "tel_aviv" {
		t.Errorf("listing[0].City = %q, want tel_aviv", listings[0].City)
	}
}

func TestEnricher_FailedAttemptsConsumeBudget(t *testing.T) {
	var requestCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	enricher := newTestEnricher(t, handler, EnricherConfig{
		Delay:       time.Millisecond,
		MaxPerCycle: 2,
	})

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
		{Token: "c", Km: 0},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all failed)", count)
	}
	if got := requestCount.Load(); got != 2 {
		t.Errorf("requests = %d, want 2 (budget=2, failures count)", got)
	}
}
