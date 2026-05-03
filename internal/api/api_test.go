package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	fbauth "firebase.google.com/go/v4/auth"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

type fakeTokenVerifier struct {
	uid    string
	email  string
	err    error
}

func (f *fakeTokenVerifier) VerifyIDToken(_ context.Context, _ string) (*fbauth.Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &fbauth.Token{
		UID:    f.uid,
		Claims: map[string]interface{}{"email": f.email},
	}, nil
}

func setupTestServer(t *testing.T) (*Server, *sqlite.Store) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cat := catalog.NewDynamic(store, slog.Default())
	cat.Load(context.Background())

	cat.Ingest(context.Background(), 19, "Toyota", 10226, "Corolla")
	cat.Ingest(context.Background(), 8, "Honda", 10061, "Civic")

	srv := New(Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		LinkTokens: store,
		Prices:   store,
		Admin:    store,
		Saved:    store,
		Hidden:   store,
		Notifs:   store,
		Logger:   slog.Default(),
		BotUsername: "carwatch_test_bot",
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
			AdminChatID: 999,
		},
	})

	if err := store.UpsertUser(context.Background(), 999, "testuser"); err != nil {
		t.Fatal(err)
	}

	return srv, store
}

func TestTelegramLinkAndStatus(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	w := doRequest(t, srv, "GET", "/api/v1/telegram/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var st map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if connected, _ := st["connected"].(bool); connected {
		t.Fatalf("expected not connected, got %v", st)
	}

	w = doRequest(t, srv, "POST", "/api/v1/telegram/link", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("link: %d %s", w.Code, w.Body.String())
	}
	var linkResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &linkResp); err != nil {
		t.Fatal(err)
	}
	link, _ := linkResp["link"].(string)
	wantPrefix := "https://t.me/carwatch_test_bot?start=link_"
	if !strings.HasPrefix(link, wantPrefix) {
		t.Fatalf("link: %s", link)
	}
	if exp, ok := linkResp["expires_in_seconds"].(float64); !ok || int(exp) != 900 {
		t.Fatalf("expires_in_seconds: %v", linkResp["expires_in_seconds"])
	}

	rawToken := strings.TrimPrefix(link, wantPrefix)
	webID, err := store.ConsumeLinkToken(ctx, rawToken)
	if err != nil {
		t.Fatal(err)
	}
	if webID != 999 {
		t.Fatalf("web chat id: %d", webID)
	}

	if err := store.UpsertUser(ctx, 42, "tguser"); err != nil {
		t.Fatal(err)
	}
	if err := store.LinkTelegramToWeb(ctx, 42, 999); err != nil {
		t.Fatal(err)
	}

	w = doRequest(t, srv, "GET", "/api/v1/telegram/status", nil)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if connected, _ := st["connected"].(bool); !connected {
		t.Fatalf("expected connected: %v", st)
	}
	if st["telegram_username"] != "tguser" {
		t.Fatalf("username: %v", st)
	}
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
	req := createSearchRequest{
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
	if searches[0].ListingsCount != 0 {
		t.Fatalf("expected listings_count 0, got %d", searches[0].ListingsCount)
	}

	// Get search
	w = doRequest(t, srv, "GET", "/api/v1/searches/"+itoa(created.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}

	// Update search
	updateReq := updateSearchRequest{
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
	req := createSearchRequest{
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
		SearchID:     created.ID,
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
		SearchID:     created.ID,
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
	if len(resp.Items) == 0 {
		t.Fatal("sort: expected items, got none")
	}
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

	// Bookmark + saved flag + listings_count on search list
	w = doRequest(t, srv, "POST", "/api/v1/listings/tok1/save", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("bookmark: expected 204, got %d", w.Code)
	}
	w = doRequest(t, srv, "GET", "/api/v1/searches/"+itoa(created.ID)+"/listings?sort=newest", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list after bookmark: %d", w.Code)
	}
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	var tok1Saved, tok2Saved bool
	for _, it := range resp.Items {
		switch it.Token {
		case "tok1":
			tok1Saved = it.Saved
		case "tok2":
			tok2Saved = it.Saved
		}
	}
	if !tok1Saved || tok2Saved {
		t.Fatalf("saved flags: tok1=%v tok2=%v, want true false", tok1Saved, tok2Saved)
	}

	w = doRequest(t, srv, "GET", "/api/v1/searches", nil)
	var searches []searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &searches)
	if len(searches) != 1 || searches[0].ListingsCount != 2 {
		t.Fatalf("search listings_count: got %+v", searches)
	}
}

func TestCreateSearch_InvalidRanges(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2025,
		YearMax:      2020,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for year_min > year_max, got %d: %s", w.Code, w.Body.String())
	}

	for _, tc := range []struct {
		name string
		req  createSearchRequest
	}{
		{"negative year_min", createSearchRequest{Manufacturer: 19, Model: 10226, YearMin: -1}},
		{"negative year_max", createSearchRequest{Manufacturer: 19, Model: 10226, YearMax: -1}},
		{"negative price_max", createSearchRequest{Manufacturer: 19, Model: 10226, PriceMax: -1}},
		{"negative max_km", createSearchRequest{Manufacturer: 19, Model: 10226, MaxKm: -1}},
		{"negative max_hand", createSearchRequest{Manufacturer: 19, Model: 10226, MaxHand: -1}},
		{"negative engine_min_cc", createSearchRequest{Manufacturer: 19, Model: 10226, EngineMinCC: -1}},
	} {
		w = doRequest(t, srv, "POST", "/api/v1/searches", tc.req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}
}

func TestUpdateSearch_InvalidRanges(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2018,
		YearMax:      2024,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	w = doRequest(t, srv, "PUT", "/api/v1/searches/"+itoa(created.ID), updateSearchRequest{
		YearMin: 2025,
		YearMax: 2020,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for year_min > year_max on update, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearchNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/searches/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPauseResumeNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches/99999/pause", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("pause nonexistent: expected 404, got %d", w.Code)
	}

	w = doRequest(t, srv, "POST", "/api/v1/searches/99999/resume", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("resume nonexistent: expected 404, got %d", w.Code)
	}
}

func TestPauseResumeForeignSearch(t *testing.T) {
	srv, store := setupTestServer(t)

	// Create a search owned by user 999
	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	// Create a second server acting as user 888
	if err := store.UpsertUser(context.Background(), 888, "other"); err != nil {
		t.Fatal(err)
	}
	cat := catalog.NewDynamic(store, slog.Default())
	cat.Load(context.Background())
	otherSrv := New(Config{
		Catalog: cat, Searches: store, Listings: store,
		Users: store, Prices: store, Logger: slog.Default(),
		API: config.APIConfig{CORSOrigins: []string{"http://localhost:5173"}, DevChatID: 888},
	})

	// User 888 tries to pause user 999's search — should get 404
	w = doRequest(t, otherSrv, "POST", "/api/v1/searches/"+itoa(created.ID)+"/pause", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("pause foreign: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, otherSrv, "POST", "/api/v1/searches/"+itoa(created.ID)+"/resume", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("resume foreign: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

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

func TestCORS_DisallowedOrigin(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/v1/searches", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS preflight, got %d", w.Code)
	}
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("expected no CORS origin header for disallowed origin, got %q", origin)
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

type failingAdminStore struct {
	failDBFileSize bool
	failTableSizes bool
}

func (f *failingAdminStore) DBFileSize() (int64, error) {
	if f.failDBFileSize {
		return 0, fmt.Errorf("disk error")
	}
	return 0, nil
}

func (f *failingAdminStore) CountAllListings(_ context.Context) (int64, error) {
	return 0, nil
}

func (f *failingAdminStore) TableSizes(_ context.Context) (map[string]int64, error) {
	if f.failTableSizes {
		return nil, fmt.Errorf("query error")
	}
	return map[string]int64{}, nil
}

func (f *failingAdminStore) PurgeTable(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (f *failingAdminStore) AdminListListings(_ context.Context, _, _ int) ([]storage.ListingRecord, int64, error) {
	return nil, 0, nil
}

func (f *failingAdminStore) AdminDeleteListing(_ context.Context, _ string, _ int64) error {
	return nil
}

func (f *failingAdminStore) VacuumDB(_ context.Context) error {
	return nil
}

func TestAdminStats_DBFileSizeError(t *testing.T) {
	srv, _ := setupTestServer(t)
	srv.admin = &failingAdminStore{failDBFileSize: true}

	w := doRequest(t, srv, "GET", "/api/v1/admin/stats", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminStats_TableSizesError(t *testing.T) {
	srv, _ := setupTestServer(t)
	srv.admin = &failingAdminStore{failTableSizes: true}

	w := doRequest(t, srv, "GET", "/api/v1/admin/stats", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetListing_Success(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-abc", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021,
		Price: 100000, Km: 50000, Hand: 2, City: "Tel Aviv",
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/listings/tok-abc", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp listingResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Token != "tok-abc" {
		t.Errorf("token = %q, want tok-abc", resp.Token)
	}
	if resp.Manufacturer != "Toyota" {
		t.Errorf("manufacturer = %q, want Toyota", resp.Manufacturer)
	}
	if resp.SearchName != "s1" {
		t.Errorf("search_name = %q, want s1", resp.SearchName)
	}
}

func TestGetListing_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/listings/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBookmarkCRUD(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-bm", ChatID: 999, SearchName: "test",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	// Save bookmark
	w := doRequest(t, srv, "POST", "/api/v1/listings/tok-bm/save", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("save: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// List saved
	w = doRequest(t, srv, "GET", "/api/v1/saved", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list saved: expected 200, got %d", w.Code)
	}
	var savedResp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &savedResp)
	if savedResp.Total != 1 {
		t.Fatalf("expected 1 saved, got %d", savedResp.Total)
	}
	if savedResp.Items[0].Token != "tok-bm" {
		t.Errorf("token = %q, want tok-bm", savedResp.Items[0].Token)
	}
	if savedResp.Items[0].SearchName != "test" {
		t.Errorf("search_name = %q, want test", savedResp.Items[0].SearchName)
	}
	if !savedResp.Items[0].Saved {
		t.Error("expected saved item to have saved=true")
	}

	w = doRequest(t, srv, "GET", "/api/v1/listings/tok-bm", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get listing: expected 200, got %d", w.Code)
	}
	var one listingResponse
	mustUnmarshal(t, w.Body.Bytes(), &one)
	if !one.Saved {
		t.Error("expected get listing saved=true")
	}

	// Remove bookmark
	w = doRequest(t, srv, "DELETE", "/api/v1/listings/tok-bm/save", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("unsave: expected 204, got %d", w.Code)
	}

	// Verify removed
	w = doRequest(t, srv, "GET", "/api/v1/saved", nil)
	mustUnmarshal(t, w.Body.Bytes(), &savedResp)
	if savedResp.Total != 0 {
		t.Errorf("expected 0 saved after removal, got %d", savedResp.Total)
	}
}

func TestGetListing_WrongOwner(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-other", ChatID: 777, SearchName: "s1",
		Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000,
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/listings/tok-other", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong owner, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListHistory(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-h1", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-h2", ChatID: 999, SearchName: "s2",
		Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000,
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Errorf("expected 2 history items, got %d", resp.Total)
	}
}

func TestHideUnhideListing(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-hide", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	// Hide
	w := doRequest(t, srv, "POST", "/api/v1/listings/tok-hide/hide", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("hide: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify hidden
	hidden, err := store.IsHidden(ctx, 999, "tok-hide")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("listing should be hidden after hide")
	}

	// Unhide
	w = doRequest(t, srv, "DELETE", "/api/v1/listings/tok-hide/hide", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("unhide: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify unhidden
	hidden, err = store.IsHidden(ctx, 999, "tok-hide")
	if err != nil {
		t.Fatal(err)
	}
	if hidden {
		t.Error("listing should not be hidden after unhide")
	}
}

func TestListHistory_Pagination(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	for i := range 5 {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: fmt.Sprintf("tok-hist-%d", i), ChatID: 999, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020 + i, Price: 100000 + i*10000,
		}); err != nil {
			t.Fatal(err)
		}
	}

	w := doRequest(t, srv, "GET", "/api/v1/history?limit=2&offset=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Total)
	}

	w = doRequest(t, srv, "GET", "/api/v1/history?limit=2&offset=4", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 {
		t.Errorf("expected 1 item at offset 4, got %d", len(resp.Items))
	}
}

func TestListSaved_Pagination(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	for i := range 3 {
		token := fmt.Sprintf("tok-sv-%d", i)
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: 999, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveBookmark(ctx, 999, token); err != nil {
			t.Fatal(err)
		}
	}

	w := doRequest(t, srv, "GET", "/api/v1/saved?limit=2&offset=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}
}

func TestAdminStats(t *testing.T) {
	srv, store := setupTestServer(t)

	ctx := context.Background()
	_, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       999,
		Name:         "test-search",
		Source:       "yad2",
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     150000,
		Active:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token:      "tok-1",
		ChatID:     999,
		SearchName: "test-search",
		Manufacturer: "Toyota",
		Model:        "Corolla",
		Year:         2020,
		Price:        120000,
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/admin/stats", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminStatsResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)

	if resp.Tables["users"] < 1 {
		t.Errorf("expected at least 1 user, got %d", resp.Tables["users"])
	}
	if resp.Tables["searches"] < 1 {
		t.Errorf("expected at least 1 search, got %d", resp.Tables["searches"])
	}
	if resp.Tables["listing_history"] < 1 {
		t.Errorf("expected at least 1 listing, got %d", resp.Tables["listing_history"])
	}
	if resp.Runtime.Goroutines < 1 {
		t.Error("expected goroutines > 0")
	}
	if resp.Runtime.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestAdminStats_NonAdmin(t *testing.T) {
	srv, store := setupTestServer(t)
	if err := store.UpsertUser(context.Background(), 888, "other"); err != nil {
		t.Fatal(err)
	}
	otherSrv := New(Config{
		Catalog: srv.catalog, Searches: srv.searches, Listings: srv.listings,
		Users: srv.users, Prices: srv.prices, Admin: srv.admin,
		Logger: slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   888,
			AdminChatID: 999,
		},
	})
	w := doRequest(t, otherSrv, "GET", "/api/v1/admin/stats", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", w.Code)
	}
}

func TestAdminStats_FirebaseAdmin(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cat := catalog.NewDynamic(store, slog.Default())
	cat.Load(context.Background())

	srv := New(Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Admin:    store,
		Logger:   slog.Default(),
		FirebaseAuth: &fakeTokenVerifier{uid: "admin-uid", email: "admin@example.com"},
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			AdminEmail:  "admin@example.com",
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for Firebase admin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminStats_FirebaseNonAdmin(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cat := catalog.NewDynamic(store, slog.Default())
	cat.Load(context.Background())

	srv := New(Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Admin:    store,
		Logger:   slog.Default(),
		FirebaseAuth: &fakeTokenVerifier{uid: "user-uid", email: "user@example.com"},
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			AdminEmail:  "admin@example.com",
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for Firebase non-admin, got %d: %s", w.Code, w.Body.String())
	}
}

func setLastSeenAt(t *testing.T, store *sqlite.Store, chatID int64, when time.Time) {
	t.Helper()
	db := store.DB()
	if _, err := db.Exec("UPDATE users SET last_seen_at = ? WHERE chat_id = ?", when, chatID); err != nil {
		t.Fatalf("set last_seen_at: %v", err)
	}
}

func TestNotificationCount(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	// Set last_seen_at to now so no listings are new
	if err := store.UpdateLastSeenAt(ctx, 999); err != nil {
		t.Fatal(err)
	}
	// Then move it back to the past so future inserts appear as new
	setLastSeenAt(t, store, 999, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	w := doRequest(t, srv, "GET", "/api/v1/notifications/count", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp notifCountResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Count != 0 {
		t.Errorf("expected 0 initially, got %d", resp.Count)
	}

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "notif-1", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	w = doRequest(t, srv, "GET", "/api/v1/notifications/count", nil)
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Count != 1 {
		t.Errorf("expected 1 after adding listing, got %d", resp.Count)
	}
}

func TestNotificationsList(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	setLastSeenAt(t, store, 999, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "notif-list-1", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/notifications", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("expected 1 notification, got %d", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(resp.Items))
	}
}

func TestNotificationsMarkSeen(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	setLastSeenAt(t, store, 999, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "notif-seen-1", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	var countResp notifCountResponse
	w := doRequest(t, srv, "GET", "/api/v1/notifications/count", nil)
	mustUnmarshal(t, w.Body.Bytes(), &countResp)
	if countResp.Count != 1 {
		t.Fatalf("expected 1 before mark seen, got %d", countResp.Count)
	}

	w = doRequest(t, srv, "POST", "/api/v1/notifications/seen", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("mark seen: expected 204, got %d", w.Code)
	}

	w = doRequest(t, srv, "GET", "/api/v1/notifications/count", nil)
	mustUnmarshal(t, w.Body.Bytes(), &countResp)
	if countResp.Count != 0 {
		t.Errorf("expected 0 after mark seen, got %d", countResp.Count)
	}
}
