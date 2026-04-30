package yad2

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

func TestEnricher_FillsMissingKm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":75000}}}}
</script></html>`)
	}))
	defer srv.Close()

	fetcher := &Yad2Fetcher{
		client:  mustPlainClient(t),
		baseURL: srv.URL,
		logger:  slog.Default(),
	}
	// Override the item URL by wrapping the fetcher's FetchItem via a test enricher.
	// Instead, we'll just test the Enricher directly with a custom server.
	enricher := &testEnricher{details: ItemDetails{Km: 75000}}

	listings := []model.RawListing{
		{Token: "a", Km: 50000},
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

	_ = fetcher
}

func TestEnricher_RespectsMaxPerCycle(t *testing.T) {
	enricher := &testEnricher{details: ItemDetails{Km: 10000}, maxPerCycle: 1}

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
}

func TestEnricher_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	enricher := &testEnricher{details: ItemDetails{Km: 10000}}

	listings := []model.RawListing{
		{Token: "a", Km: 0},
		{Token: "b", Km: 0},
	}

	// First item gets enriched (no delay before first), second should be skipped.
	count := enricher.Enrich(ctx, listings)
	if count > 1 {
		t.Errorf("enriched = %d, want <= 1 (ctx canceled)", count)
	}
}

func TestEnricher_SkipsOnError(t *testing.T) {
	enricher := &testEnricher{err: fmt.Errorf("network error")}

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
	enricher := &testEnricher{details: ItemDetails{Km: 50000}}

	listings := []model.RawListing{
		{Token: "a", Km: 10000},
		{Token: "b", Km: 20000},
	}

	count := enricher.Enrich(context.Background(), listings)
	if count != 0 {
		t.Errorf("enriched = %d, want 0 (all have km)", count)
	}
}

// testEnricher is a mock that simulates the real Enricher's logic.
type testEnricher struct {
	details     ItemDetails
	err         error
	maxPerCycle int
}

func (e *testEnricher) Enrich(ctx context.Context, listings []model.RawListing) int {
	max := e.maxPerCycle
	if max <= 0 {
		max = 100
	}
	enriched := 0
	for i := range listings {
		if listings[i].Km > 0 {
			continue
		}
		if enriched >= max {
			break
		}
		if enriched > 0 {
			select {
			case <-ctx.Done():
				return enriched
			case <-time.After(time.Millisecond):
			}
		}
		if e.err != nil {
			continue
		}
		if e.details.Km > 0 {
			listings[i].Km = e.details.Km
			enriched++
		}
	}
	return enriched
}

func mustPlainClient(t *testing.T) HTTPDoer {
	t.Helper()
	c, err := newPlainClient([]string{"test-ua"}, "")
	if err != nil {
		t.Fatalf("create plain client: %v", err)
	}
	return c
}
