package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

// fakeFetcher returns canned listings for testing.
type fakeFetcher struct {
	listings []model.RawListing
}

func (f *fakeFetcher) Fetch(_ context.Context, _ model.SourceParams) ([]model.RawListing, error) {
	return f.listings, nil
}

func setupGuestTestServer(t *testing.T, listings []model.RawListing) *Server {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cat := catalog.NewDynamic(slog.Default())
	cat.Load(context.Background())
	cat.Ingest(context.Background(), catalog.IngestEntry{
		ManufacturerID: 19, ManufacturerName: "Toyota",
		ManufacturerNameHe: "טויוטה",
		ModelID:            10226, ModelName: "Corolla", ModelNameHe: "קורולה",
	})

	factory := fetcher.NewFactory()
	factory.Register("yad2", &fakeFetcher{listings: listings})

	srv := New(Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		Fetchers: factory,
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
		},
	})

	if err := store.UpsertUser(context.Background(), 999, "testuser"); err != nil {
		t.Fatal(err)
	}

	return srv
}

func doGuestRequest(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header — guest request.

	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	return w
}

func makeTestListings(n int) []model.RawListing {
	listings := make([]model.RawListing, n)
	for i := range n {
		listings[i] = model.RawListing{
			Token:          "tok-" + itoa(int64(i)),
			Manufacturer:   "Toyota",
			ManufacturerID: 19,
			Model:          "Corolla",
			ModelID:        10226,
			Year:           2020,
			Price:          100000 + i*1000,
			Km:             50000 + i*1000,
			Hand:           1,
			City:           "Tel Aviv",
			PageLink:       "https://yad2.co.il/item/tok-" + itoa(int64(i)),
		}
	}
	return listings
}

func TestInstantSearch_HappyPath(t *testing.T) {
	listings := makeTestListings(5)
	srv := setupGuestTestServer(t, listings)

	req := instantSearchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     200000,
	}

	w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp instantSearchResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 5 {
		t.Fatalf("expected total 5, got %d", resp.Total)
	}
	if len(resp.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(resp.Items))
	}

	// Verify fitness_score is present on each item.
	for i, item := range resp.Items {
		if item.FitnessScore == nil {
			t.Errorf("item[%d] missing fitness_score", i)
		}
	}

	// Verify sorted by fitness score descending.
	for i := 1; i < len(resp.Items); i++ {
		if *resp.Items[i].FitnessScore > *resp.Items[i-1].FitnessScore {
			t.Errorf("items not sorted by fitness_score desc: [%d]=%v > [%d]=%v",
				i, *resp.Items[i].FitnessScore, i-1, *resp.Items[i-1].FitnessScore)
		}
	}
}

func TestInstantSearch_MissingManufacturer(t *testing.T) {
	srv := setupGuestTestServer(t, nil)

	req := instantSearchRequest{
		Source: "yad2",
		Model:  10226,
	}

	w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing manufacturer, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInstantSearch_InvalidYearRange(t *testing.T) {
	srv := setupGuestTestServer(t, nil)

	req := instantSearchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		YearMin:      2025,
		YearMax:      2020,
	}

	w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid year range, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInstantSearch_CappedAt30(t *testing.T) {
	listings := makeTestListings(50)
	srv := setupGuestTestServer(t, listings)

	req := instantSearchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     999999,
	}

	w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp instantSearchResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 30 {
		t.Fatalf("expected total capped at 30, got %d", resp.Total)
	}
	if len(resp.Items) != 30 {
		t.Fatalf("expected 30 items, got %d", len(resp.Items))
	}
}

func TestInstantSearch_RateLimited(t *testing.T) {
	listings := makeTestListings(1)
	srv := setupGuestTestServer(t, listings)

	req := instantSearchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     200000,
	}

	// Exhaust the guest rate limit (burst=15).
	for i := 0; i < 15; i++ {
		w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	// 16th request should be rate limited.
	w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("16th request: expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInstantSearch_NoAuthRequired(t *testing.T) {
	// Verify that the guest endpoint works without any auth,
	// and that auth-only endpoints still require auth.
	srv := setupGuestTestServer(t, makeTestListings(1))

	// Guest endpoint should work without auth.
	req := instantSearchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     200000,
	}
	w := doGuestRequest(t, srv, "POST", "/api/v1/guest/instant-search", req)
	if w.Code != http.StatusOK {
		t.Fatalf("guest endpoint should work without auth, got %d: %s", w.Code, w.Body.String())
	}

	// Catalog endpoints should also work without auth via guest chain.
	w = doGuestRequest(t, srv, "GET", "/api/v1/catalog/manufacturers", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("catalog should work without auth, got %d: %s", w.Code, w.Body.String())
	}
}
