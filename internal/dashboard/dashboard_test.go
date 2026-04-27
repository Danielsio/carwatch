package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestDashboard_Empty(t *testing.T) {
	store := newTestStore(t)
	h := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Showing 0 listings") {
		t.Error("should show 0 listings")
	}
}

func TestDashboard_WithListings(t *testing.T) {
	store := newTestStore(t)

	_ = store.SaveListing(context.Background(), storage.ListingRecord{
		Token: "abc", ChatID: 100, SearchName: "test", Manufacturer: "Mazda", Model: "3",
		Year: 2021, Price: 95000, Km: 85000, Hand: 2, City: "Tel Aviv",
		PageLink: "https://example.com/abc",
	})

	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Mazda") {
		t.Error("should contain Mazda")
	}
	if !strings.Contains(body, "95,000") {
		t.Error("should contain formatted price")
	}
	if !strings.Contains(body, "Showing 1 listings") {
		t.Error("should show 1 listing")
	}
}

func TestDashboard_LimitParam(t *testing.T) {
	store := newTestStore(t)

	for i := range 5 {
		_ = store.SaveListing(context.Background(), storage.ListingRecord{
			Token: string(rune('a' + i)), ChatID: int64(100 + i), SearchName: "test",
			Manufacturer: "Test", Model: "Car", Year: 2021, Price: 100000,
		})
	}

	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/dashboard?limit=2", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "Showing 2 listings") {
		t.Error("limit=2 should show 2 listings")
	}
}

func TestDashboard_InvalidLimitFallsBackToDefault(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	for i := range 3 {
		_ = store.SaveListing(ctx, storage.ListingRecord{
			Token: string(rune('a' + i)), ChatID: int64(100 + i), SearchName: "test",
			Manufacturer: "Test", Model: "Car", Year: 2021, Price: 100000,
		})
	}
	h := NewHandler(store)

	for _, q := range []string{"?limit=abc", "?limit=-5", "?limit=1000"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dashboard"+q, nil))

		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", q, w.Code)
		}
		if !strings.Contains(w.Body.String(), "Showing 3 listings") {
			t.Errorf("%s: invalid limit should fall back to default and show all 3 listings", q)
		}
	}
}

type errorStore struct{}

func (e *errorStore) ListListings(_ context.Context, _ int) ([]storage.ListingRecord, error) {
	return nil, errors.New("db error")
}

func TestDashboard_StoreError(t *testing.T) {
	h := NewHandler(&errorStore{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestDashboard_SecurityHeaders(t *testing.T) {
	store := newTestStore(t)
	h := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	headers := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	}
	for k, v := range headers {
		if got := w.Header().Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
	if csp := w.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("Content-Security-Policy header not set")
	}
}

