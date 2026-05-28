package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
)

type mockFetcher struct {
	listings []model.RawListing
	err      error
	calls    int
	mu       sync.Mutex
}

func (m *mockFetcher) Fetch(_ context.Context, _ model.SourceParams) ([]model.RawListing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.listings, m.err
}

type partialFetcher struct {
	listings []model.RawListing
	err      error
	calls    int
}

func (m *partialFetcher) Fetch(_ context.Context, _ model.SourceParams) ([]model.RawListing, error) {
	m.calls++
	return m.listings, m.err
}

type dedupKey struct {
	token  string
	chatID int64
}

type mockDedup struct {
	seen map[dedupKey]bool
	mu   sync.Mutex
}

func newMockDedup() *mockDedup {
	return &mockDedup{seen: make(map[dedupKey]bool)}
}

func (m *mockDedup) ClaimNew(_ context.Context, token string, chatID int64, _ int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := dedupKey{token, chatID}
	if m.seen[key] {
		return false, nil
	}
	m.seen[key] = true
	return true, nil
}

func (m *mockDedup) ReleaseClaim(_ context.Context, token string, chatID int64, _ int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.seen, dedupKey{token, chatID})
	return nil
}

func (m *mockDedup) Prune(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockDedup) Close() error { return nil }

type mockNotifier struct {
	messages    []notifyCall
	rawMessages []rawNotifyCall
	err         error
	mu          sync.Mutex
}

type notifyCall struct {
	recipient string
	count     int
}

type rawNotifyCall struct {
	recipient string
	message   string
}

func (m *mockNotifier) Connect(_ context.Context) error { return nil }
func (m *mockNotifier) Disconnect() error               { return nil }

func (m *mockNotifier) NotifyRaw(_ context.Context, recipient string, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rawMessages = append(m.rawMessages, rawNotifyCall{recipient: recipient, message: message})
	return nil
}

func (m *mockNotifier) Notify(_ context.Context, recipient string, listings []model.Listing, _ locale.Lang) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.messages = append(m.messages, notifyCall{recipient: recipient, count: len(listings)})
	return nil
}

type mockPriceTracker struct {
	prices map[string]int
	mu     sync.Mutex
}

func newMockPriceTracker() *mockPriceTracker {
	return &mockPriceTracker{prices: make(map[string]int)}
}

func (m *mockPriceTracker) RecordPrice(_ context.Context, token string, price int) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, exists := m.prices[token]
	m.prices[token] = price
	if exists && price < old {
		return old, true, nil
	}
	return 0, false, nil
}

func (m *mockPriceTracker) RevertPrice(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.prices, token)
	return nil
}

func (m *mockPriceTracker) PrunePrices(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockPriceTracker) GetPriceHistory(_ context.Context, _ string) ([]storage.PricePoint, error) {
	return nil, nil
}

func testConfig() *config.Config {
	return &config.Config{
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
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestFetchWithRetryUsing_Success(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{{Token: "a"}}}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	listings, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{}, s.logger)
	if err != nil {
		t.Fatalf("fetchWithRetryUsing: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing, got %d", len(listings))
	}
}

func TestFetchWithRetryUsing_ChallengeNoRetry(t *testing.T) {
	f := &mockFetcher{err: fmt.Errorf("yad2: %w", fetcher.ErrChallenge)}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	_, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{}, s.logger)
	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Errorf("expected ErrChallenge, got: %v", err)
	}
	if f.calls != 1 {
		t.Errorf("challenge should not retry, got %d calls", f.calls)
	}
}

func TestFetchWithRetryUsing_PartialResults_ReturnsListings(t *testing.T) {
	partial := &partialFetcher{
		listings: []model.RawListing{{Token: "a"}, {Token: "b"}},
		err:      fmt.Errorf("%w: page 3: timeout", fetcher.ErrPartialResults),
	}
	cfg := testConfig()
	s, _ := New(cfg, partial, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	listings, err := s.fetchWithRetryUsing(ctx, partial, model.SourceParams{}, s.logger)
	if err != nil {
		t.Errorf("partial results should be returned as success, got: %v", err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 partial listings, got %d", len(listings))
	}
	if partial.calls != 1 {
		t.Errorf("partial results should not retry, got %d calls", partial.calls)
	}
}

func TestFetchWithRetryUsing_CircuitOpenNoRetry(t *testing.T) {
	f := &mockFetcher{err: fetcher.ErrCircuitOpen}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	_, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{}, s.logger)
	if !errors.Is(err, fetcher.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
	if f.calls != 1 {
		t.Errorf("circuit open should not retry, got %d calls", f.calls)
	}
}

func TestFetchWithRetryUsing_RetriesOnError(t *testing.T) {
	f := &mockFetcher{err: errors.New("timeout")}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	_, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{}, s.logger)
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	if f.calls != 3 {
		t.Errorf("expected 3 retry attempts, got %d", f.calls)
	}
}

func TestNextDelay_WithBackoff(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.Interval = 10 * time.Minute
	cfg.Polling.Jitter = 0

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	s.backoffMultiplier = 2.0

	delay := s.nextDelay()
	if delay != 20*time.Minute {
		t.Errorf("delay = %v, want 20m (10m * 2.0 backoff)", delay)
	}
}

func TestNextDelay_MinimumOneMinute(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.Interval = 30 * time.Second
	cfg.Polling.Jitter = 0

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	delay := s.nextDelay()
	if delay < time.Minute {
		t.Errorf("delay = %v, minimum should be 1 minute", delay)
	}
}

func TestIsActiveHours_NoConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.ActiveHours = nil

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	if !s.isActiveHours() {
		t.Error("should be active when no active hours configured")
	}
}

func TestIsActiveHours_WithinWindow(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.ActiveHours = &config.ActiveHours{
		Start: "00:00",
		End:   "23:59",
	}

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	if !s.isActiveHours() {
		t.Error("should be active within 00:00-23:59")
	}
}

func TestRunMultiTenantCycle_ObserverSuccessPath(t *testing.T) {
	store := pgtest.NewStore(t)

	ctx := context.Background()
	if err := store.UpsertUser(ctx, 1, "alice"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if _, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       1,
		Name:         "s1",
		Source:       "yad2",
		Manufacturer: 19,
		Model:        1,
		YearMin:      2015,
		YearMax:      2025,
		PriceMax:     200_000,
	}); err != nil {
		t.Fatalf("create search: %v", err)
	}

	f := &mockFetcher{listings: []model.RawListing{{
		Token:          "tok-observer-1",
		ManufacturerID: 19,
		ModelID:        1,
		Manufacturer:   "Toyota",
		Model:          "Corolla",
		Year:           2018,
		Price:          90_000,
	}}}

	cfg := testConfig()
	obs := &countingObserver{}
	n := &mockNotifier{}
	s, err := NewWithOptions(cfg, f, store, n, testLogger(), Options{
		Observer:     obs,
		SearchStore:  store,
		ListingStore: store,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("runMultiTenantCycle: %v", err)
	}

	if v := obs.fetches.Load(); v != 1 {
		t.Errorf("RecordFetch calls = %d, want 1", v)
	}
	if v := obs.listingsFound.Load(); v != 1 {
		t.Errorf("listings aggregate = %d, want 1", v)
	}
	if v := obs.notifications.Load(); v != 1 {
		t.Errorf("notifications = %d, want 1", v)
	}
	if v := obs.successes.Load(); v != 1 {
		t.Errorf("successes = %d, want 1", v)
	}
	if v := obs.errors.Load(); v != 0 {
		t.Errorf("errors = %d, want 0", v)
	}
}

// --- mockMarketStore ---

type mockMarketStore struct {
	mu    sync.Mutex
	rows  []storage.MarketMedianRow
	calls int // counts LoadMarketMedians calls
}

func (m *mockMarketStore) RefreshMarketMedians(_ context.Context) error {
	return nil
}

func (m *mockMarketStore) LoadMarketMedians(_ context.Context) ([]storage.MarketMedianRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.rows, nil
}

func TestMarketCacheReusedAcrossCycles(t *testing.T) {
	ms := &mockMarketStore{
		rows: []storage.MarketMedianRow{
			{Manufacturer: "toyota", Model: "corolla", Year: 2020, MedianPrice: 110000, MedianKm: 50000, CohortSize: 15},
		},
	}

	f := &mockFetcher{listings: []model.RawListing{}}
	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 19, Model: 1,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
		},
	}
	cfg := testConfig()

	s, err := NewWithOptions(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), Options{
		SearchStore:    ss,
		MarketStore:    ms,
		MarketCacheTTL: 1 * time.Hour, // Long TTL so cache stays fresh.
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First cycle builds the cache.
	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	ms.mu.Lock()
	calls1 := ms.calls
	ms.mu.Unlock()
	if calls1 != 1 {
		t.Fatalf("expected 1 MarketListings call after first cycle, got %d", calls1)
	}

	// Second cycle reuses the cached market data.
	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle 2: %v", err)
	}
	ms.mu.Lock()
	calls2 := ms.calls
	ms.mu.Unlock()
	if calls2 != 1 {
		t.Errorf("expected cache reuse (still 1 call), got %d MarketListings calls", calls2)
	}
}

func TestMarketCacheInvalidatedOnNewListings(t *testing.T) {
	ms := &mockMarketStore{
		rows: []storage.MarketMedianRow{
			{Manufacturer: "mazda", Model: "3", Year: 2020, MedianPrice: 95000, MedianKm: 55000, CohortSize: 15},
		},
	}

	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-mc-1", ManufacturerID: 27, ModelID: 10332,
				Manufacturer: "Mazda", Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	ls := &mockListingStore{}
	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}
	cfg := testConfig()

	s, err := NewWithOptions(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), Options{
		SearchStore:    ss,
		MarketStore:    ms,
		ListingStore:   ls,
		MarketCacheTTL: 1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First cycle: builds cache and saves new listings (which invalidates it).
	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	ms.mu.Lock()
	calls1 := ms.calls
	ms.mu.Unlock()
	if calls1 != 1 {
		t.Fatalf("expected 1 MarketListings call after first cycle, got %d", calls1)
	}

	// Verify cache was invalidated (marketCache should be nil).
	s.marketCacheMu.RLock()
	cacheNil := s.marketCache == nil
	s.marketCacheMu.RUnlock()
	if !cacheNil {
		t.Error("market cache should be invalidated after new listings are saved")
	}

	// Second cycle: should rebuild the cache since it was invalidated.
	// Reset the fetcher to return no new listings so we can observe the rebuild.
	f.mu.Lock()
	f.listings = []model.RawListing{}
	f.mu.Unlock()

	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle 2: %v", err)
	}
	ms.mu.Lock()
	calls2 := ms.calls
	ms.mu.Unlock()
	if calls2 != 2 {
		t.Errorf("expected 2 MarketListings calls (cache invalidated), got %d", calls2)
	}
}

func TestRunMultiTenantCycle_ObserverErrorOnFetchFailure(t *testing.T) {
	store := pgtest.NewStore(t)

	ctx := context.Background()
	if err := store.UpsertUser(ctx, 2, "bob"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if _, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       2,
		Name:         "s2",
		Source:       "yad2",
		Manufacturer: 27,
		Model:        1,
	}); err != nil {
		t.Fatalf("create search: %v", err)
	}

	f := &mockFetcher{err: errors.New("fetch failed")}
	cfg := testConfig()
	obs := &countingObserver{}
	s, err := NewWithOptions(cfg, f, store, &mockNotifier{}, testLogger(), Options{
		Observer:    obs,
		SearchStore: store,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	if err := s.runMultiTenantCycle(ctx); err == nil {
		t.Fatal("expected error when all fetch groups fail")
	}
	if v := obs.errors.Load(); v != 1 {
		t.Errorf("errors = %d, want 1", v)
	}
	if v := obs.fetches.Load(); v != 1 {
		t.Errorf("fetches = %d, want 1", v)
	}
	if v := obs.successes.Load(); v != 0 {
		t.Errorf("successes = %d, want 0", v)
	}
}
