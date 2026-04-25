//go:build e2e

package e2e

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// --- mock notifier ---

type mockNotifier struct {
	messages    []model.Listing
	rawMessages []string
}

func (m *mockNotifier) Connect(_ context.Context) error    { return nil }
func (m *mockNotifier) Disconnect() error                  { return nil }

func (m *mockNotifier) Notify(_ context.Context, _ string, listings []model.Listing, _ locale.Lang) error {
	m.messages = append(m.messages, listings...)
	return nil
}

func (m *mockNotifier) NotifyRaw(_ context.Context, _ string, msg string) error {
	m.rawMessages = append(m.rawMessages, msg)
	return nil
}

// --- stub fetcher ---

type stubFetcher struct {
	listings []model.RawListing
}

func (f *stubFetcher) Fetch(_ context.Context, _ config.SourceParams) ([]model.RawListing, error) {
	return f.listings, nil
}

// TestE2E_FullPipeline tests the complete pipeline:
// store → search creation → fetch → dedup → notify
func TestE2E_FullPipeline(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "testuser"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       100,
		Name:         "e2e-mazda3",
		Source:       "yad2",
		Manufacturer: 27,
		Model:        10332,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     150000,
		EngineMinCC:  1800,
		Active:       true,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}
	if searchID == 0 {
		t.Fatal("expected non-zero search ID")
	}

	n := &mockNotifier{}
	f := &stubFetcher{
		listings: []model.RawListing{
			{
				Token:        "e2e-token-1",
				Manufacturer: "Mazda",
				Model:        "3",
				Year:         2021,
				Price:        95000,
				Km:           60000,
				Hand:         2,
				EngineVolume: 2000,
				City:         "Tel Aviv",
				PageLink:     "https://example.com/1",
			},
			{
				Token:        "e2e-token-2",
				Manufacturer: "Mazda",
				Model:        "3",
				Year:         2020,
				Price:        85000,
				Km:           80000,
				Hand:         3,
				EngineVolume: 1500,
				City:         "Haifa",
				PageLink:     "https://example.com/2",
			},
		},
	}

	h := health.New()
	h.SetUserCounter(store)
	h.SetSearchCounter(store)

	cfg := &config.Config{
		Polling: config.PollingConfig{
			Interval: 1 * time.Minute,
			Jitter:   0,
			Timezone: "UTC",
		},
		Telegram: config.TelegramConfig{
			Token: "test-token",
		},
		Storage: config.StorageConfig{
			PruneAfter: 24 * time.Hour,
		},
	}

	factory := fetcher.NewFactory()
	factory.Register("yad2", f)

	sched, err := scheduler.NewWithOptions(cfg, f, store, n, testLogger, scheduler.Options{
		Observer:       h,
		Queue:          store,
		Prices:         store,
		FetcherFactory: factory,
		ListingStore:   store,
		SearchStore:    store,
		DigestStore:    store,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	schedCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = sched.Run(schedCtx)

	if len(n.messages) == 0 {
		t.Error("expected at least one notification")
	}

	foundFiltered := false
	for _, m := range n.messages {
		if m.Token == "e2e-token-2" {
			foundFiltered = true
		}
	}
	if foundFiltered {
		t.Error("e2e-token-2 (1500cc) should be filtered out by EngineMinCC=1800")
	}

	found := false
	for _, m := range n.messages {
		if m.Token == "e2e-token-1" {
			found = true
		}
	}
	if !found {
		t.Error("e2e-token-1 should be in notifications")
	}

	// Verify dedup: running again should not re-notify
	n2 := &mockNotifier{}
	sched2, _ := scheduler.NewWithOptions(cfg, f, store, n2, testLogger, scheduler.Options{
		Observer:       h,
		Queue:          store,
		Prices:         store,
		FetcherFactory: factory,
		ListingStore:   store,
		SearchStore:    store,
		DigestStore:    store,
	})
	schedCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()
	_ = sched2.Run(schedCtx2)

	if len(n2.messages) != 0 {
		t.Errorf("expected 0 notifications on second run (dedup), got %d", len(n2.messages))
	}
}

// TestE2E_CatalogStore tests catalog persistence round-trip
func TestE2E_CatalogStore(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	entries := []storage.CatalogEntry{
		{ManufacturerID: 27, ManufacturerName: "Mazda", ModelID: 10332, ModelName: "3"},
		{ManufacturerID: 27, ManufacturerName: "Mazda", ModelID: 10342, ModelName: "CX-5"},
		{ManufacturerID: 19, ManufacturerName: "Toyota", ModelID: 10226, ModelName: "Corolla"},
	}

	if err := store.SaveCatalogEntries(ctx, entries); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded))
	}

	age, err := store.CatalogAge(ctx)
	if err != nil {
		t.Fatalf("age: %v", err)
	}
	if age > 5*time.Second {
		t.Errorf("catalog age should be < 5s, got %v", age)
	}
}

// TestE2E_StaticCatalog verifies the static catalog contains expected data
func TestE2E_StaticCatalog(t *testing.T) {
	cat := catalog.NewStatic()

	mfrs := cat.Manufacturers()
	if len(mfrs) < 10 {
		t.Fatalf("expected at least 10 manufacturers, got %d", len(mfrs))
	}

	mazda := cat.Models(27)
	if len(mazda) == 0 {
		t.Fatal("Mazda should have models")
	}

	found := false
	for _, m := range mazda {
		if m.Name == "3" {
			found = true
		}
	}
	if !found {
		t.Error("Mazda 3 not found in static catalog")
	}
}

// TestE2E_HealthEndpoint tests the health HTTP endpoint
func TestE2E_HealthEndpoint(t *testing.T) {
	h := health.New()
	h.RecordSuccess()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Handler())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// TestE2E_DigestFlow tests the full digest workflow
func TestE2E_DigestFlow(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "digestuser"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	if err := store.SetDigestMode(ctx, 100, "digest", "1h"); err != nil {
		t.Fatalf("set digest mode: %v", err)
	}

	mode, interval, err := store.GetDigestMode(ctx, 100)
	if err != nil {
		t.Fatalf("get digest mode: %v", err)
	}
	if mode != "digest" || interval != "1h" {
		t.Errorf("mode=%q interval=%q, want digest/1h", mode, interval)
	}

	if err := store.AddDigestItem(ctx, 100, "listing A"); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if err := store.AddDigestItem(ctx, 100, "listing B"); err != nil {
		t.Fatalf("add item: %v", err)
	}

	users, err := store.PendingDigestUsers(ctx)
	if err != nil {
		t.Fatalf("pending users: %v", err)
	}
	if len(users) != 1 || users[0] != 100 {
		t.Errorf("pending users = %v, want [100]", users)
	}

	payloads, cutoff, err := store.PeekDigest(ctx, 100)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if len(payloads) != 2 {
		t.Errorf("expected 2 payloads, got %d", len(payloads))
	}

	if err := store.AckDigest(ctx, 100, cutoff); err != nil {
		t.Fatalf("ack: %v", err)
	}

	payloads, _, _ = store.PeekDigest(ctx, 100)
	if len(payloads) != 0 {
		t.Error("peek after ack should be empty")
	}
}

// TestE2E_PriceTracking tests price drop detection
func TestE2E_PriceTracking(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	_, changed, _ := store.RecordPrice(ctx, "tok-1", 100000)
	if changed {
		t.Error("first price should not be a change")
	}

	oldPrice, changed, _ := store.RecordPrice(ctx, "tok-1", 90000)
	if !changed || oldPrice != 100000 {
		t.Errorf("price drop: changed=%v oldPrice=%d", changed, oldPrice)
	}

	_, changed, _ = store.RecordPrice(ctx, "tok-1", 95000)
	if changed {
		t.Error("price increase should not trigger change")
	}
}
