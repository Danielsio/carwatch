package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

type mockFetcher struct {
	listings []model.RawListing
	err      error
	calls    int
	mu       sync.Mutex
}

func (m *mockFetcher) Fetch(_ context.Context, _ config.SourceParams) ([]model.RawListing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockNotifier) Notify(_ context.Context, recipient string, listings []model.Listing) error {
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

func testConfig() *config.Config {
	return &config.Config{
		Polling: config.PollingConfig{
			Interval: 1 * time.Minute,
			Jitter:   0,
			Timezone: "UTC",
		},
		Searches: []config.SearchConfig{
			{
				Name:       "test-search",
				Source:     "yad2",
				Recipients: []string{"+972123456789"},
			},
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

func TestProcessSearch_NewListings(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000},
			{Token: "b", Manufacturer: "Mazda", Model: "3", Price: 85000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	if err := s.processSearch(ctx, cfg.Searches[0]); err != nil {
		t.Fatalf("processSearch: %v", err)
	}

	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notify call, got %d", len(n.messages))
	}
	if n.messages[0].count != 2 {
		t.Errorf("expected 2 listings, got %d", n.messages[0].count)
	}
	if !d.seen[dedupKey{"a", 0}] || !d.seen[dedupKey{"b", 0}] {
		t.Error("tokens should be marked as seen")
	}
}

func TestProcessSearch_AllSeen(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a"},
		},
	}
	d := newMockDedup()
	d.seen[dedupKey{"a", 0}] = true
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	if err := s.processSearch(ctx, cfg.Searches[0]); err != nil {
		t.Fatalf("processSearch: %v", err)
	}

	if len(n.messages) != 0 {
		t.Error("no notifications expected for seen listings")
	}
}

func TestProcessSearch_NotifyFailure_ReleasesClaims(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a"},
			{Token: "b"},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{err: errors.New("whatsapp down")}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	if err := s.processSearch(ctx, cfg.Searches[0]); err != nil {
		t.Fatalf("processSearch: %v", err)
	}

	if d.seen[dedupKey{"a", 0}] || d.seen[dedupKey{"b", 0}] {
		t.Error("claims should be released after notify failure")
	}
}

func TestProcessSearch_PartialNotifySuccess(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{{Token: "a"}},
	}
	d := newMockDedup()
	cfg := testConfig()
	cfg.Searches[0].Recipients = []string{"+972111", "+972222"}

	customN := &conditionalNotifier{
		failOn: "+972111",
	}

	s, _ := New(cfg, f, d, customN, testLogger(), nil)
	ctx := context.Background()

	if err := s.processSearch(ctx, cfg.Searches[0]); err != nil {
		t.Fatalf("processSearch: %v", err)
	}

	if !d.seen[dedupKey{"a", 0}] {
		t.Error("claim should be kept when at least one recipient succeeds")
	}
}

type conditionalNotifier struct {
	failOn string
}

func (c *conditionalNotifier) Connect(_ context.Context) error                       { return nil }
func (c *conditionalNotifier) Disconnect() error                                     { return nil }
func (c *conditionalNotifier) NotifyRaw(_ context.Context, _ string, _ string) error { return nil }
func (c *conditionalNotifier) Notify(_ context.Context, recipient string, _ []model.Listing) error {
	if recipient == c.failOn {
		return fmt.Errorf("failed for %s", recipient)
	}
	return nil
}

func TestProcessSearch_FetchError(t *testing.T) {
	f := &mockFetcher{err: errors.New("network error")}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	err := s.processSearch(ctx, cfg.Searches[0])
	if err == nil {
		t.Fatal("expected error on fetch failure")
	}
}

func TestProcessSearch_ChallengeError(t *testing.T) {
	f := &mockFetcher{err: fmt.Errorf("yad2: %w", fetcher.ErrChallenge)}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	err := s.processSearch(ctx, cfg.Searches[0])
	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Errorf("expected ErrChallenge, got: %v", err)
	}
}

func TestRunCycle_AllSearchesFail(t *testing.T) {
	f := &mockFetcher{err: errors.New("down")}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	err := s.runCycle(ctx)
	if err == nil {
		t.Fatal("expected error when all searches fail")
	}
}

func TestRunCycle_ChallengeIncreasesBackoff(t *testing.T) {
	f := &mockFetcher{err: fmt.Errorf("yad2: %w", fetcher.ErrChallenge)}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	ctx := context.Background()

	initialBackoff := s.backoffMultiplier
	_ = s.runCycle(ctx)

	if s.backoffMultiplier <= initialBackoff {
		t.Errorf("backoff should increase on challenge: was %f, now %f", initialBackoff, s.backoffMultiplier)
	}
}

func TestRunCycle_SuccessDecreasesBackoff(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	s, _ := New(cfg, f, d, n, testLogger(), nil)
	s.backoffMultiplier = 4.0
	ctx := context.Background()

	_ = s.runCycle(ctx)

	if s.backoffMultiplier >= 4.0 {
		t.Errorf("backoff should decrease on success: %f", s.backoffMultiplier)
	}
}

func TestFetchWithRetry_Success(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{{Token: "a"}}}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	listings, err := s.fetchWithRetry(ctx, config.SourceParams{})
	if err != nil {
		t.Fatalf("fetchWithRetry: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing, got %d", len(listings))
	}
}

func TestFetchWithRetry_ChallengeNoRetry(t *testing.T) {
	f := &mockFetcher{err: fmt.Errorf("yad2: %w", fetcher.ErrChallenge)}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	_, err := s.fetchWithRetry(ctx, config.SourceParams{})
	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Errorf("expected ErrChallenge, got: %v", err)
	}
	if f.calls != 1 {
		t.Errorf("challenge should not retry, got %d calls", f.calls)
	}
}

func TestFetchWithRetry_RetriesOnError(t *testing.T) {
	f := &mockFetcher{err: errors.New("timeout")}
	cfg := testConfig()
	s, _ := New(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	ctx := context.Background()
	_, err := s.fetchWithRetry(ctx, config.SourceParams{})
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

func TestProcessSearch_PriceDropNotification(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 89000, Km: 85000},
		},
	}
	d := newMockDedup()
	// Pre-mark token as seen so the pagination pre-scan stops after page 0,
	// avoiding duplicate copies of the same listing in allRaw.
	d.seen[dedupKey{"a", 0}] = true
	n := &mockNotifier{}
	cfg := testConfig()

	pt := newMockPriceTracker()
	pt.prices["a"] = 95000

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{Prices: pt})
	ctx := context.Background()

	if err := s.processSearch(ctx, cfg.Searches[0]); err != nil {
		t.Fatalf("processSearch: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.messages) != 0 {
		t.Errorf("expected 0 Notify calls for price drop (should use NotifyRaw), got %d", len(n.messages))
	}

	if len(n.rawMessages) != 1 {
		t.Fatalf("expected 1 NotifyRaw call for price drop, got %d", len(n.rawMessages))
	}

	msg := n.rawMessages[0].message
	if !strings.Contains(msg, "Price Drop!") {
		t.Errorf("price drop message should contain 'Price Drop!', got:\n%s", msg)
	}
	if !strings.Contains(msg, "₪95,000") || !strings.Contains(msg, "₪89,000") {
		t.Errorf("price drop message should contain old and new price, got:\n%s", msg)
	}
	if !strings.Contains(msg, "-₪6,000") {
		t.Errorf("price drop message should contain drop amount, got:\n%s", msg)
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
