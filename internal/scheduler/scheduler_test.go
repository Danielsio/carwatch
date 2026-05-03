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
	"github.com/dsionov/carwatch/internal/storage/sqlite"
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

func (m *mockDedup) ReleaseClaim(_ context.Context, token string, chatID int64) error {
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
	listings, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{})
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
	_, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{})
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
	listings, err := s.fetchWithRetryUsing(ctx, partial, model.SourceParams{})
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
	_, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{})
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
	_, err := s.fetchWithRetryUsing(ctx, f, model.SourceParams{})
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

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+972501234567", "+97250123****"},
		{"+1234", "+****"},
		{"123", "***"},
		{"", "***"},
	}

	for _, tt := range tests {
		got := maskPhone(tt.input)
		if got != tt.want {
			t.Errorf("maskPhone(%q) = %q, want %q", tt.input, got, tt.want)
		}
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
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	defer func() { _ = store.Close() }()

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

	if obs.fetches != 1 {
		t.Errorf("RecordFetch calls = %d, want 1", obs.fetches)
	}
	if obs.listingsFound != 1 {
		t.Errorf("listings aggregate = %d, want 1", obs.listingsFound)
	}
	if obs.notifications != 1 {
		t.Errorf("notifications = %d, want 1", obs.notifications)
	}
	if obs.successes != 1 {
		t.Errorf("successes = %d, want 1", obs.successes)
	}
	if obs.errors != 0 {
		t.Errorf("errors = %d, want 0", obs.errors)
	}
}

func TestRunMultiTenantCycle_ObserverErrorOnFetchFailure(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	defer func() { _ = store.Close() }()

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
	if obs.errors != 1 {
		t.Errorf("errors = %d, want 1", obs.errors)
	}
	if obs.fetches != 1 {
		t.Errorf("fetches = %d, want 1", obs.fetches)
	}
	if obs.successes != 0 {
		t.Errorf("successes = %d, want 0", obs.successes)
	}
}
