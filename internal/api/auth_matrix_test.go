package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
)

func TestAuthMatrix_CrossUserDenied(t *testing.T) {
	store := pgtest.NewStore(t)
	ctx := context.Background()

	cat := catalog.NewDynamic(slog.Default())
	cat.Load(ctx)
	cat.Ingest(ctx, catalog.IngestEntry{ManufacturerID: 19, ManufacturerName: "Toyota", ModelID: 10226, ModelName: "Corolla"})

	// User A (owner) — creates resources
	if err := store.UpsertUser(ctx, 999, "owner"); err != nil {
		t.Fatal(err)
	}
	// User B (attacker) — tries to access them
	if err := store.UpsertUser(ctx, 888, "attacker"); err != nil {
		t.Fatal(err)
	}

	ownerSrv := New(Config{
		Catalog: cat, Searches: store, Listings: store, Users: store,
		Prices: store, Admin: store, Saved: store, Hidden: store,
		Notifs: store, PushSubs: store, LinkTokens: store,
		Logger: slog.Default(),
		BotUsername: "test_bot",
		API: config.APIConfig{
			CORSOrigins: []string{"*"}, DevChatID: 999, AdminChatID: 999,
		},
		Push: config.PushConfig{VAPIDPublicKey: "test"},
	})

	attackerSrv := New(Config{
		Catalog: cat, Searches: store, Listings: store, Users: store,
		Prices: store, Admin: store, Saved: store, Hidden: store,
		Notifs: store, PushSubs: store, LinkTokens: store,
		Logger: slog.Default(),
		BotUsername: "test_bot",
		API: config.APIConfig{
			CORSOrigins: []string{"*"}, DevChatID: 888, AdminChatID: 999,
		},
		Push: config.PushConfig{VAPIDPublicKey: "test"},
	})

	// Seed: create search owned by user 999
	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 999, Name: "owner-search", Source: "yad2",
		Manufacturer: 19, Model: 10226, YearMin: 2020, YearMax: 2025, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Seed: listing owned by user 999
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "owner-tok", ChatID: 999, SearchID: searchID, SearchName: "owner-search",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		Km: 50000, Hand: 2, City: "Tel Aviv", FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	// Seed: bookmark + push sub for user 999
	_ = store.SaveBookmark(ctx, 999, "owner-tok")
	_ = store.SavePushSubscription(ctx, storage.PushSubscription{
		ChatID: 999, Endpoint: "https://push.example/sub", P256DH: "k", Auth: "a",
	})

	sid := fmt.Sprintf("%d", searchID)

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		// Search operations
		{"GET search", "GET", "/api/v1/searches/" + sid, 404},
		{"PUT search", "PUT", "/api/v1/searches/" + sid, 404},
		{"DELETE search", "DELETE", "/api/v1/searches/" + sid, 404},
		{"pause search", "POST", "/api/v1/searches/" + sid + "/pause", 404},
		{"resume search", "POST", "/api/v1/searches/" + sid + "/resume", 404},
		{"search stats", "GET", "/api/v1/searches/" + sid + "/stats", 404},
		{"search listings", "GET", "/api/v1/searches/" + sid + "/listings", 404},

		// Listing operations
		{"GET listing", "GET", "/api/v1/listings/owner-tok", 404},

		// Admin operations (user 888 is not admin)
		{"admin stats", "GET", "/api/v1/admin/stats", 403},
		{"admin listings", "GET", "/api/v1/admin/listings", 403},
		{"admin searches", "GET", "/api/v1/admin/searches", 403},
		{"admin users", "GET", "/api/v1/admin/users", 403},
		{"admin vacuum", "POST", "/api/v1/admin/vacuum", 403},
		{"admin purge", "POST", "/api/v1/admin/purge", 403},
		{"admin sync", "POST", "/api/v1/admin/sync-user-status", 403},
		{"admin activity", "GET", "/api/v1/admin/activity", 403},
		{"admin delete listing", "DELETE", "/api/v1/admin/listings/owner-tok", 403},
		{"admin delete search", "DELETE", "/api/v1/admin/searches/" + sid, 403},
		{"admin delete user", "DELETE", "/api/v1/admin/users/999", 403},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			attackerSrv.Routes().ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Errorf("%s %s: got %d, want %d (body: %s)",
					tc.method, tc.path, w.Code, tc.want, w.Body.String())
			}
		})
	}

	// Verify owner CAN access the same resources
	ownerChecks := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"owner GET search", "GET", "/api/v1/searches/" + sid, 200},
		{"owner search stats", "GET", "/api/v1/searches/" + sid + "/stats", 200},
		{"owner search listings", "GET", "/api/v1/searches/" + sid + "/listings", 200},
		{"owner GET listing", "GET", "/api/v1/listings/owner-tok", 200},
		{"owner admin stats", "GET", "/api/v1/admin/stats", 200},
	}

	for _, tc := range ownerChecks {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			ownerSrv.Routes().ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Errorf("%s %s: got %d, want %d (body: %s)",
					tc.method, tc.path, w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestAuthMatrix_UnauthenticatedDenied(t *testing.T) {
	store := pgtest.NewStore(t)
	ctx := context.Background()

	cat := catalog.NewDynamic(slog.Default())
	cat.Load(ctx)

	// Server with auth token required (no DevChatID fallback)
	srv := New(Config{
		Catalog: cat, Searches: store, Listings: store, Users: store,
		Prices: store, Admin: store, Saved: store, Hidden: store,
		Notifs: store, PushSubs: store, LinkTokens: store,
		Logger: slog.Default(),
		BotUsername: "test_bot",
		API: config.APIConfig{
			CORSOrigins: []string{"*"}, DevChatID: 999, AuthToken: "secret",
		},
		Push: config.PushConfig{VAPIDPublicKey: "test"},
	})

	strictRoutes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/searches"},
		{"GET", "/api/v1/searches/1"},
		{"PUT", "/api/v1/searches/1"},
		{"DELETE", "/api/v1/searches/1"},
		{"POST", "/api/v1/searches/1/pause"},
		{"POST", "/api/v1/searches/1/resume"},
		{"GET", "/api/v1/searches/1/stats"},
		{"GET", "/api/v1/searches/1/listings"},
		{"GET", "/api/v1/listings/tok1"},
		{"GET", "/api/v1/listings/tok1/price-history"},
		{"GET", "/api/v1/telegram/status"},
		{"POST", "/api/v1/telegram/link"},
		{"POST", "/api/v1/listings/tok1/save"},
		{"DELETE", "/api/v1/listings/tok1/save"},
		{"GET", "/api/v1/saved"},
		{"POST", "/api/v1/listings/tok1/hide"},
		{"DELETE", "/api/v1/listings/tok1/hide"},
		{"GET", "/api/v1/history"},
		{"GET", "/api/v1/notifications"},
		{"POST", "/api/v1/notifications/seen"},
		{"POST", "/api/v1/listings/tok1/seen"},
		{"DELETE", "/api/v1/listings/tok1/seen"},
		{"POST", "/api/v1/push/subscribe"},
		{"DELETE", "/api/v1/push/subscribe"},
		{"GET", "/api/v1/push/vapid-key"},
		{"GET", "/api/v1/admin/stats"},
	}

	for _, tc := range strictRoutes {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			srv.Routes().ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: got %d, want 401", tc.method, tc.path, w.Code)
			}
		})
	}

	// Optional-auth routes return 200 even without auth (guest mode)
	optionalRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/me"},
		{"GET", "/api/v1/searches"},
		{"GET", "/api/v1/notifications/count"},
	}

	for _, tc := range optionalRoutes {
		t.Run("guest_"+tc.method+"_"+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			srv.Routes().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("guest %s %s: got %d, want 200", tc.method, tc.path, w.Code)
			}
		})
	}

	// Public routes return 200 without auth
	publicRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/catalog/manufacturers"},
	}

	for _, tc := range publicRoutes {
		t.Run("public_"+tc.method+"_"+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			srv.Routes().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("public %s %s: got %d, want 200", tc.method, tc.path, w.Code)
			}
		})
	}
}
