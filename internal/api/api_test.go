package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func setupTestServer(t *testing.T) (*Server, *sqlite.Store) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	cat := catalog.NewDynamic(store, slog.Default())
	cat.Load(context.Background())

	cat.Ingest(context.Background(), 19, "Toyota", 10226, "Corolla")
	cat.Ingest(context.Background(), 8, "Honda", 10061, "Civic")

	srv := New(Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
		},
	})

	if err := store.UpsertUser(context.Background(), 999, "testuser"); err != nil {
		t.Fatal(err)
	}

	return srv, store
}

func doRequest(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
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

	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	return w
}

func TestListManufacturers(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/catalog/manufacturers", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var mfrs []catalogEntry
	if err := json.Unmarshal(w.Body.Bytes(), &mfrs); err != nil {
		t.Fatal(err)
	}
	if len(mfrs) < 2 {
		t.Fatalf("expected at least 2 manufacturers, got %d", len(mfrs))
	}
}

func TestListManufacturers_Search(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/catalog/manufacturers?q=Toy", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var mfrs []catalogEntry
	mustUnmarshal(t, w.Body.Bytes(), &mfrs)
	if len(mfrs) != 1 || mfrs[0].Name != "Toyota" {
		t.Fatalf("expected Toyota, got %v", mfrs)
	}
}

func TestListModels(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/catalog/manufacturers/19/models", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var models []catalogEntry
	mustUnmarshal(t, w.Body.Bytes(), &models)
	found := false
	for _, m := range models {
		if m.Name == "Corolla" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Corolla in models, got %v", models)
	}
}

func TestSearchCRUD(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create search
	req := searchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     200000,
	}
	w := doRequest(t, srv, "POST", "/api/v1/searches", req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)
	if created.ManufacturerName != "Toyota" {
		t.Fatalf("expected Toyota, got %s", created.ManufacturerName)
	}
	if !created.Active {
		t.Fatal("expected active search")
	}

	// List searches
	w = doRequest(t, srv, "GET", "/api/v1/searches", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	var searches []searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &searches)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}

	// Get search
	w = doRequest(t, srv, "GET", "/api/v1/searches/"+itoa(created.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}

	// Update search
	updateReq := searchRequest{
		YearMin:  2020,
		YearMax:  2026,
		PriceMax: 300000,
		MaxKm:    100000,
	}
	w = doRequest(t, srv, "PUT", "/api/v1/searches/"+itoa(created.ID), updateReq)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &updated)
	if updated.PriceMax != 300000 {
		t.Fatalf("expected price_max 300000, got %d", updated.PriceMax)
	}

	// Pause search
	w = doRequest(t, srv, "POST", "/api/v1/searches/"+itoa(created.ID)+"/pause", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("pause: expected 204, got %d", w.Code)
	}

	// Resume search
	w = doRequest(t, srv, "POST", "/api/v1/searches/"+itoa(created.ID)+"/resume", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("resume: expected 204, got %d", w.Code)
	}

	// Delete search
	w = doRequest(t, srv, "DELETE", "/api/v1/searches/"+itoa(created.ID), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", w.Code)
	}

	// Verify deleted
	w = doRequest(t, srv, "GET", "/api/v1/searches", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("verify delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	mustUnmarshal(t, w.Body.Bytes(), &searches)
	if len(searches) != 0 {
		t.Fatalf("expected 0 searches after delete, got %d", len(searches))
	}
}

func TestListListings(t *testing.T) {
	srv, store := setupTestServer(t)

	// Create a search
	req := searchRequest{
		Source:       "yad2",
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     200000,
	}
	w := doRequest(t, srv, "POST", "/api/v1/searches", req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	// Add listings
	ctx := context.Background()
	score := 7.5
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token:        "tok1",
		ChatID:       999,
		SearchName:   created.Name,
		Manufacturer: "Toyota",
		Model:        "Corolla",
		Year:         2020,
		Price:        120000,
		Km:           50000,
		Hand:         2,
		City:         "Tel Aviv",
		PageLink:     "https://yad2.co.il/item/tok1",
		ImageURL:     "https://img.yad2.co.il/tok1.jpg",
		FitnessScore: &score,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token:        "tok2",
		ChatID:       999,
		SearchName:   created.Name,
		Manufacturer: "Toyota",
		Model:        "Corolla",
		Year:         2022,
		Price:        180000,
		Km:           20000,
		Hand:         1,
		City:         "Haifa",
		PageLink:     "https://yad2.co.il/item/tok2",
	}); err != nil {
		t.Fatal(err)
	}

	// List listings
	w = doRequest(t, srv, "GET", "/api/v1/searches/"+itoa(created.ID)+"/listings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Fatalf("expected 2 listings, got %d", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}

	// Test sort by price_asc
	w = doRequest(t, srv, "GET", "/api/v1/searches/"+itoa(created.ID)+"/listings?sort=price_asc", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("sort: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Items[0].Price != 120000 {
		t.Fatalf("expected cheapest first, got %d", resp.Items[0].Price)
	}

	// Test pagination
	w = doRequest(t, srv, "GET", "/api/v1/searches/"+itoa(created.ID)+"/listings?limit=1&offset=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pagination: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Total != 2 {
		t.Fatalf("expected 1 item, total 2, got %d items, total %d", len(resp.Items), resp.Total)
	}
}

func TestSearchNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/searches/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAuthMiddleware(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cat := catalog.NewDynamic(store, slog.Default())

	srv := New(Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
			AuthToken:   "secret123",
		},
	})

	// No token — should fail
	w := doRequest(t, srv, "GET", "/api/v1/searches", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}

	// Wrong token — should fail
	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w2, req)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", w2.Code)
	}

	// Correct token — should succeed
	if err := store.UpsertUser(context.Background(), 999, "testuser"); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest("GET", "/api/v1/searches", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	w3 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w3, req)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct token, got %d: %s", w3.Code, w3.Body.String())
	}
}

func TestCORS(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/v1/searches", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("expected CORS origin header, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}
