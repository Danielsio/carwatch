package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
)

// --- getMe ---

func TestGetMe(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/v1/me", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp["is_admin"] != true {
		t.Errorf("dev user 999 should be admin, got %v", resp)
	}
}

// --- adminSetUserActive ---

func TestAdminSetUserActive(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()
	if err := store.UpsertUser(ctx, 200, "bob"); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "PATCH", "/api/v1/admin/users/200", map[string]bool{"active": false})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	u, err := store.GetUser(ctx, 200)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.Active {
		t.Error("user should be inactive after PATCH")
	}
}

func TestAdminSetUserActive_BadID(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "PATCH", "/api/v1/admin/users/abc", map[string]bool{"active": true})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- adminGetLogLevel / adminSetLogLevel ---

func TestAdminLogLevel_GetAndSet(t *testing.T) {
	srv, _ := setupTestServer(t)

	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	srv.logLevel = lvl

	w := doRequest(t, srv, "GET", "/api/v1/admin/logs/level", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d", w.Code)
	}
	var resp map[string]string
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp["level"] == "" {
		t.Error("expected a level string")
	}

	w = doRequest(t, srv, "PUT", "/api/v1/admin/logs/level", map[string]string{"level": "WARN"})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT WARN: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, srv, "PUT", "/api/v1/admin/logs/level", map[string]string{"level": "INVALID"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("PUT INVALID: expected 400, got %d", w.Code)
	}

	w = doRequest(t, srv, "PUT", "/api/v1/admin/logs/level", map[string]string{"level": "INFO"})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT INFO reset: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- adminListPriceHistory ---

func TestAdminListPriceHistory(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	db := store.DB()
	if _, err := db.ExecContext(ctx, "INSERT INTO price_history (token, price) VALUES ('ph-1', 100000)"); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/admin/price-history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp["total"].(float64) < 1 {
		t.Error("expected at least 1 price record")
	}
}

// --- adminListSeenListings ---

func TestAdminListSeenListings(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 999, Name: "seen-admin", Manufacturer: 1, Model: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimNew(ctx, "admin-seen-tok", 999, searchID); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/admin/seen-listings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- adminActivity ---

func TestAdminActivity(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/v1/admin/activity?days=7", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	items, ok := resp["items"].([]any)
	if !ok || len(items) < 7 {
		t.Errorf("expected at least 7 activity days, got %d", len(items))
	}
}

// --- listingFilterFromSearch (all combos) ---

func TestListingFilterFromSearch(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 999, Name: "filter-test", Manufacturer: 19, Model: 10226,
		YearMin: 2018, YearMax: 2024, PriceMax: 200000, MaxKm: 100000,
		MaxHand: 3, SellerFilter: "private", GearBox: "automatic",
		PriceOnly: true, PhotoOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, rec := range []storage.ListingRecord{
		{Token: "pass-1", ChatID: 999, SearchID: searchID, SearchName: "filter-test",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 150000,
			Km: 80000, Hand: 2, GearBox: "automatic", ImageURL: "https://img.com/1.jpg",
			IsCommercial: ptrBool(false), FirstSeenAt: time.Now().UTC()},
		{Token: "fail-km", ChatID: 999, SearchID: searchID, SearchName: "filter-test",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 150000,
			Km: 200000, Hand: 2, GearBox: "automatic", ImageURL: "https://img.com/2.jpg",
			IsCommercial: ptrBool(false), FirstSeenAt: time.Now().UTC()},
		{Token: "fail-hand", ChatID: 999, SearchID: searchID, SearchName: "filter-test",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 150000,
			Km: 50000, Hand: 5, GearBox: "automatic", ImageURL: "https://img.com/3.jpg",
			IsCommercial: ptrBool(false), FirstSeenAt: time.Now().UTC()},
		{Token: "fail-seller", ChatID: 999, SearchID: searchID, SearchName: "filter-test",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 150000,
			Km: 50000, Hand: 2, GearBox: "automatic", ImageURL: "https://img.com/4.jpg",
			IsCommercial: ptrBool(true), FirstSeenAt: time.Now().UTC()},
		{Token: "fail-price", ChatID: 999, SearchID: searchID, SearchName: "filter-test",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 0,
			Km: 50000, Hand: 2, GearBox: "automatic", ImageURL: "https://img.com/5.jpg",
			IsCommercial: ptrBool(false), FirstSeenAt: time.Now().UTC()},
		{Token: "fail-photo", ChatID: 999, SearchID: searchID, SearchName: "filter-test",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 150000,
			Km: 50000, Hand: 2, GearBox: "automatic", ImageURL: "",
			IsCommercial: ptrBool(false), FirstSeenAt: time.Now().UTC()},
	} {
		if err := store.SaveListing(ctx, rec); err != nil {
			t.Fatalf("save %s: %v", rec.Token, err)
		}
	}

	w := doRequest(t, srv, "GET", fmt.Sprintf("/api/v1/searches/%d/listings", searchID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)

	tokens := make(map[string]bool)
	for _, item := range resp.Items {
		tokens[item.Token] = true
	}
	if !tokens["pass-1"] {
		t.Error("pass-1 should appear")
	}
	for _, bad := range []string{"fail-km", "fail-hand", "fail-seller", "fail-price", "fail-photo"} {
		if tokens[bad] {
			t.Errorf("%s should be filtered out", bad)
		}
	}
}

func ptrBool(b bool) *bool { return &b }

// --- bookmarks: save, unsave, list, hide, unhide ---

func TestBookmarks_SaveUnsaveList(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "bm-tok", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "POST", "/api/v1/listings/bm-tok/save", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("save: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, srv, "GET", "/api/v1/saved", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list saved: expected 200, got %d", w.Code)
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("expected 1 saved, got %d", resp.Total)
	}

	w = doRequest(t, srv, "DELETE", "/api/v1/listings/bm-tok/save", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("unsave: expected 204, got %d", w.Code)
	}
}

func TestBookmarks_UnsaveMissingToken(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "DELETE", "/api/v1/listings/nonexistent-tok/save", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("unsave nonexistent should be 204, got %d", w.Code)
	}
}

func TestHideListing_HideAndUnhide(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "hide-tok", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "POST", "/api/v1/listings/hide-tok/hide", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("hide: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, srv, "DELETE", "/api/v1/listings/hide-tok/hide", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("unhide: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// --- listHistory ---

func TestListHistory_ReturnsRecords(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "hist-tok", ChatID: 999, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "GET", "/api/v1/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp listingsPageResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Total < 1 {
		t.Error("expected at least 1 history item")
	}
}

// --- createSearch / updateSearch / deleteSearch ---

func TestSearchCRUD_FullLifecycle(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Source: "yad2", Manufacturer: 19, Model: 10226,
		YearMin: 2018, YearMax: 2024, PriceMax: 200000,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created searchResponse
	mustUnmarshal(t, w.Body.Bytes(), &created)

	w = doRequest(t, srv, "PUT", fmt.Sprintf("/api/v1/searches/%d", created.ID), map[string]any{
		"year_min": 2020, "year_max": 2025, "price_max": 250000,
		"manufacturer": 19, "model": 10226, "source": "yad2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, srv, "GET", "/api/v1/searches", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []json.RawMessage
	mustUnmarshal(t, w.Body.Bytes(), &list)
	if len(list) < 1 {
		t.Error("expected at least 1 search")
	}

	w = doRequest(t, srv, "DELETE", fmt.Sprintf("/api/v1/searches/%d", created.ID), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSearch_InvalidYears(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "POST", "/api/v1/searches", createSearchRequest{
		Manufacturer: 19, Model: 10226, YearMin: 2025, YearMax: 2020,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid year range, got %d", w.Code)
	}
}

// --- notificationCount for user with no notifications ---

func TestNotificationCount_NoNotifications(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/v1/notifications/count", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp notifCountResponse
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp.Count != 0 {
		t.Errorf("user with no notifs should get 0, got %d", resp.Count)
	}
}

// --- markNotificationsSeen ---

func TestMarkNotificationsSeen(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	searchID := seedNotifSearch(t, store, 999)
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "seen-notif-1", ChatID: 999, SearchID: searchID, SearchName: "notif-s",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "POST", "/api/v1/notifications/seen", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// --- admin: requireAdmin rejects non-admin ---

func TestRequireAdmin_RejectsNonAdmin(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()
	if err := store.UpsertUser(ctx, 888, "nonadmin"); err != nil {
		t.Fatal(err)
	}

	otherSrv := New(Config{
		Catalog: srv.catalog, Searches: srv.searches, Listings: srv.listings,
		Users: srv.users, Prices: srv.prices, Admin: srv.admin,
		Saved: srv.saved, Hidden: srv.hidden, Notifs: srv.notifs,
		Logger: srv.logger,
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   888,
			AdminChatID: 999,
		},
	})

	w := doRequest(t, otherSrv, "GET", "/api/v1/admin/stats", nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("non-admin should get 403, got %d", w.Code)
	}
}

// --- writeJSON error path ---

func TestWriteJSON_SuccessPath(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"hello": "world"})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	mustUnmarshal(t, w.Body.Bytes(), &resp)
	if resp["hello"] != "world" {
		t.Errorf("expected world, got %s", resp["hello"])
	}
}
