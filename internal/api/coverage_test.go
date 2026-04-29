package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// --- parsePagination edge cases ---

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	limit, offset := parsePagination(r)
	if limit != 20 {
		t.Errorf("default limit = %d, want 20", limit)
	}
	if offset != 0 {
		t.Errorf("default offset = %d, want 0", offset)
	}
}

func TestParsePagination_OverMax(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?limit=200", nil)
	limit, _ := parsePagination(r)
	if limit != 100 {
		t.Errorf("limit capped = %d, want 100", limit)
	}
}

func TestParsePagination_NegativeLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?limit=-5", nil)
	limit, _ := parsePagination(r)
	if limit != 20 {
		t.Errorf("negative limit = %d, want 20", limit)
	}
}

func TestParsePagination_NegativeOffset(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?offset=-10", nil)
	_, offset := parsePagination(r)
	if offset != 0 {
		t.Errorf("negative offset = %d, want 0", offset)
	}
}

func TestParsePagination_ZeroLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?limit=0", nil)
	limit, _ := parsePagination(r)
	if limit != 20 {
		t.Errorf("zero limit = %d, want 20 (default)", limit)
	}
}

// --- parseSortParam ---

func TestParseSortParam(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"price_asc", "price_asc"},
		{"price_desc", "price_desc"},
		{"score", "score"},
		{"km", "km"},
		{"year", "year"},
		{"newest", "newest"},
		{"", "newest"},
		{"invalid", "newest"},
	} {
		r := httptest.NewRequest("GET", "/test?sort="+tc.input, nil)
		got := parseSortParam(r)
		if got != tc.want {
			t.Errorf("parseSortParam(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- parsePathID ---

func TestParsePathID_Invalid(t *testing.T) {
	for _, val := range []string{"abc", "0", "-1", ""} {
		r := httptest.NewRequest("GET", "/test", nil)
		r.SetPathValue("id", val)
		_, ok := parsePathID(r)
		if ok {
			t.Errorf("parsePathID(%q) should return false", val)
		}
	}
}

// --- humanBytes ---

func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{2684354560, "2.5 GB"},
	} {
		got := humanBytes(tc.input)
		if got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- splitKeywords ---

func TestSplitKeywords(t *testing.T) {
	if got := splitKeywords("  hello  "); got != "hello" {
		t.Errorf("splitKeywords = %q, want %q", got, "hello")
	}
	if got := splitKeywords(""); got != "" {
		t.Errorf("splitKeywords empty = %q, want empty", got)
	}
}

// --- API: create search with invalid body ---

func TestCreateSearch_InvalidBody(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/searches", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", w.Code)
	}
}

func TestCreateSearch_MissingManufacturerModel(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing manufacturer/model, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSearch_UnknownManufacturer_AcceptsWithFallback(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 999999, Model: 10226,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (catalog falls back to 'Unknown'), got %d", w.Code)
	}
	var resp searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.ManufacturerName != "Unknown" {
		t.Errorf("manufacturer_name = %q, want Unknown", resp.ManufacturerName)
	}
}

func TestCreateSearch_UnknownModel_AcceptsWithFallback(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 999999,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (catalog falls back to 'Unknown'), got %d", w.Code)
	}
	var resp searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.ModelName != "Unknown" {
		t.Errorf("model_name = %q, want Unknown", resp.ModelName)
	}
}

// --- API: update search with invalid body ---

func TestUpdateSearch_InvalidBody(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024,
	})
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	req := httptest.NewRequest("PUT", "/api/v1/searches/"+itoa(created.ID), strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", rec.Code)
	}
}

func TestUpdateSearch_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "PUT", "/api/v1/searches/99999", updateSearchRequest{})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent search, got %d", w.Code)
	}
}

func TestDeleteSearch_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "DELETE", "/api/v1/searches/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete nonexistent, got %d", w.Code)
	}
}

// --- API: invalid path IDs ---

func TestGetSearch_InvalidID(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/searches/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid search id, got %d", w.Code)
	}
}

func TestUpdateSearch_InvalidID(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "PUT", "/api/v1/searches/abc", updateSearchRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", w.Code)
	}
}

func TestDeleteSearch_InvalidID(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "DELETE", "/api/v1/searches/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", w.Code)
	}
}

func TestPauseSearch_InvalidID(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches/abc/pause", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResumeSearch_InvalidID(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches/abc/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListListings_InvalidSearchID(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/searches/abc/listings", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListListings_SearchNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/searches/99999/listings", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- API: listings with limit cap ---

func TestListListings_LimitCap(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024,
	})
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "lc-1", ChatID: 999, SearchName: created.Name,
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	w = doRequest(t, srv, "GET",
		"/api/v1/searches/"+itoa(created.ID)+"/listings?limit=200", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Limit != 100 {
		t.Errorf("limit should be capped to 100, got %d", resp.Limit)
	}
}

// --- API: getListing missing token ---

func TestGetListing_EmptyToken(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/listings/", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty token (mux rejects empty path param), got %d", w.Code)
	}
}

// --- API: catalog model search ---

func TestListModels_WithSearch(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/catalog/manufacturers/19/models?q=Cor", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var models []catalogEntry
	mustUnmarshal(t, w.Body.Bytes(), &models)
	if len(models) == 0 {
		t.Fatal("expected at least one model matching 'Cor'")
	}
	found := false
	for _, m := range models {
		if m.Name == "Corolla" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Corolla in results, got %v", models)
	}
}

func TestListModels_InvalidID(t *testing.T) {
	srv, _ := setupTestServer(t)

	for _, path := range []string{
		"/api/v1/catalog/manufacturers/abc/models",
		"/api/v1/catalog/manufacturers/0/models",
		"/api/v1/catalog/manufacturers/-1/models",
	} {
		w := doRequest(t, srv, "GET", path, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("path=%s: expected 400, got %d", path, w.Code)
		}
	}
}

func TestListModels_EmptyResult(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/catalog/manufacturers/19/models?q=ZZZ", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var models []catalogEntry
	mustUnmarshal(t, w.Body.Bytes(), &models)
	if len(models) != 0 {
		t.Errorf("expected empty list, got %d items", len(models))
	}
}

// --- API: notifications pagination ---

func TestNotificationsList_Pagination(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	setLastSeenAt(t, store, 999, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	for i := range 5 {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: fmt.Sprintf("notif-pg-%d", i), ChatID: 999, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000 + i*1000,
			FirstSeenAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	w := doRequest(t, srv, "GET", "/api/v1/notifications?limit=2&offset=0", nil)
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
}

// --- API: listing with fitness score ---

func TestGetListing_WithFitnessScore(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	score := 8.5
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-score", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021,
		Price: 100000, Km: 50000, Hand: 2,
		FitnessScore: &score,
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/listings/tok-score", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listingResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.FitnessScore == nil || *resp.FitnessScore != 8.5 {
		t.Errorf("fitness_score = %v, want 8.5", resp.FitnessScore)
	}
}

// --- API: auth with no DevChatID ---

func TestAuth_NoChatID(t *testing.T) {
	srv, _ := setupTestServer(t)
	srv.cfg.DevChatID = 0

	w := doRequest(t, srv, "GET", "/api/v1/searches", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no DevChatID, got %d", w.Code)
	}
}

// --- API: search creation with keywords ---

func TestCreateSearch_WithKeywords(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024,
		Keywords: "  automatic  ", ExcludeKeys: "  salvage  ",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)
	if created.Keywords != "automatic" {
		t.Errorf("keywords = %q, want trimmed", created.Keywords)
	}
	if created.ExcludeKeys != "salvage" {
		t.Errorf("exclude_keys = %q, want trimmed", created.ExcludeKeys)
	}
}

// --- API: writeJSON error path (already tested implicitly but coverage) ---

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["key"] != "value" {
		t.Errorf("resp = %v", resp)
	}
}

// --- API: update search with negative values ---

func TestUpdateSearch_NegativeValues(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024,
	})
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	for _, tc := range []struct {
		name string
		req  updateSearchRequest
	}{
		{"negative year_min", updateSearchRequest{YearMin: -1}},
		{"negative year_max", updateSearchRequest{YearMax: -1}},
		{"negative price_max", updateSearchRequest{PriceMax: -1}},
		{"negative max_km", updateSearchRequest{MaxKm: -1}},
		{"negative max_hand", updateSearchRequest{MaxHand: -1}},
		{"negative engine_min_cc", updateSearchRequest{EngineMinCC: -1}},
	} {
		w = doRequest(t, srv, "PUT", "/api/v1/searches/"+itoa(created.ID), tc.req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}
}

// --- API: all sort variants via listings ---

func TestListListings_AllSorts(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024,
	})
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	for _, tok := range []string{"sort-1", "sort-2"} {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: tok, ChatID: 999, SearchName: created.Name,
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		}); err != nil {
			t.Fatal(err)
		}
	}

	for _, sort := range []string{"price_asc", "price_desc", "score", "km", "year", "newest", "invalid"} {
		w = doRequest(t, srv, "GET",
			"/api/v1/searches/"+itoa(created.ID)+"/listings?sort="+sort, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("sort=%s: expected 200, got %d", sort, w.Code)
		}
	}
}

// --- API: create search default source ---

func TestCreateSearch_DefaultSource(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2020, YearMax: 2024,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var resp searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Source != "yad2" {
		t.Errorf("source = %q, want yad2", resp.Source)
	}
}

// --- API: CORS with GET request ---

func TestCORS_WithGETRequest(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:5173" {
		t.Errorf("expected CORS header on GET, got %q", origin)
	}
}

// --- API: history with no listings ---

func TestListHistory_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 0 {
		t.Errorf("expected 0 history, got %d", resp.Total)
	}
}

// --- API: saved with no bookmarks ---

func TestListSaved_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/saved", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 0 {
		t.Errorf("expected 0 saved, got %d", resp.Total)
	}
}

// --- API: listing with all fields ---

func TestListListings_WithAllFields(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2018, YearMax: 2024, PriceMax: 200000,
	})
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	score := 9.2
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "full-1", ChatID: 999, SearchName: created.Name,
		Manufacturer: "Toyota", Model: "Corolla", Year: 2022,
		Price: 150000, Km: 30000, Hand: 1, City: "Haifa",
		PageLink: "https://yad2.co.il/item/full-1",
		ImageURL: "https://img.yad2.co.il/full-1.jpg",
		FitnessScore: &score,
	}); err != nil {
		t.Fatal(err)
	}

	w = doRequest(t, srv, "GET",
		"/api/v1/searches/"+itoa(created.ID)+"/listings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.City != "Haifa" {
		t.Errorf("city = %q, want Haifa", item.City)
	}
	if item.ImageURL != "https://img.yad2.co.il/full-1.jpg" {
		t.Errorf("image_url = %q", item.ImageURL)
	}
	if item.FitnessScore == nil || *item.FitnessScore != 9.2 {
		t.Errorf("fitness_score = %v, want 9.2", item.FitnessScore)
	}
}
