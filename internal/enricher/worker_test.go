package enricher

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/storage"
)

type mockItemFetcher struct {
	mu      sync.Mutex
	calls   int
	details ItemDetails
	err     error
}

func (m *mockItemFetcher) FetchItem(_ context.Context, _ string) (ItemDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.details, m.err
}

type mockListingStore struct {
	mu             sync.Mutex
	enrichmentData map[string]storage.EnrichmentRecord
	backfilled     []storage.ListingRecord
	enrichAttempts map[string]int
	backfillErr    error
	lookupErr      error
	incrementErr   error
}

func newMockListingStore() *mockListingStore {
	return &mockListingStore{
		enrichmentData: make(map[string]storage.EnrichmentRecord),
		enrichAttempts: make(map[string]int),
	}
}

func (m *mockListingStore) LookupEnrichmentData(_ context.Context, tokens []string) (map[string]storage.EnrichmentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lookupErr != nil {
		return nil, m.lookupErr
	}
	result := make(map[string]storage.EnrichmentRecord)
	for _, t := range tokens {
		if rec, ok := m.enrichmentData[t]; ok {
			result[t] = rec
		}
	}
	return result, nil
}

func (m *mockListingStore) BackfillListings(_ context.Context, records []storage.ListingRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.backfillErr != nil {
		return m.backfillErr
	}
	m.backfilled = append(m.backfilled, records...)
	return nil
}

func (m *mockListingStore) IncrementEnrichAttempt(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.incrementErr != nil {
		return m.incrementErr
	}
	m.enrichAttempts[token]++
	return nil
}

func (m *mockListingStore) SaveListing(_ context.Context, _ storage.ListingRecord) error { return nil }
func (m *mockListingStore) SaveListings(_ context.Context, _ []storage.ListingRecord) error {
	return nil
}
func (m *mockListingStore) GetListing(_ context.Context, _ int64, _ string) (*storage.ListingRecord, error) {
	return nil, nil
}
func (m *mockListingStore) ListUserListings(_ context.Context, _ int64, _, _ int) ([]storage.ListingRecord, error) {
	return nil, nil
}
func (m *mockListingStore) CountUserListings(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (m *mockListingStore) ListSearchListings(_ context.Context, _ int64, _ int64, _ storage.ListingFilter, _, _ int, _ string) ([]storage.ListingRecord, error) {
	return nil, nil
}
func (m *mockListingStore) CountSearchListings(_ context.Context, _ int64, _ int64, _ storage.ListingFilter) (int64, error) {
	return 0, nil
}
func (m *mockListingStore) CountSearchListingsForChat(_ context.Context, _ int64) (map[int64]int64, error) {
	return nil, nil
}
func (m *mockListingStore) SearchStats(_ context.Context, _ int64, _ int64, _ storage.ListingFilter) (*storage.SearchStats, error) {
	return nil, nil
}
func (m *mockListingStore) ListUnenrichedTokens(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}
func (m *mockListingStore) CountUnenrichedTokens(_ context.Context) (int64, error) { return 0, nil }
func (m *mockListingStore) PruneListings(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockListingStore) DeleteStaleListings(_ context.Context, _ int64, _ int64, _ []string) (int64, error) {
	return 0, nil
}

func testWorkerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestWorker_EnrichesSuccessfully(t *testing.T) {
	f := &mockItemFetcher{details: ItemDetails{Km: 50000, City: "Tel Aviv", ImageURL: "https://img.yad2.co.il/test.jpg"}}
	ls := newMockListingStore()
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	ls.mu.Lock()
	defer ls.mu.Unlock()
	if len(ls.backfilled) != 1 {
		t.Fatalf("expected 1 backfilled record, got %d", len(ls.backfilled))
	}
	if ls.backfilled[0].Km != 50000 {
		t.Errorf("backfilled km = %d, want 50000", ls.backfilled[0].Km)
	}
	if ls.backfilled[0].City != "Tel Aviv" {
		t.Errorf("backfilled city = %q, want Tel Aviv", ls.backfilled[0].City)
	}
	if ls.enrichAttempts["tok-1"] != 0 {
		t.Errorf("enrich attempts = %d, want 0 (not incremented on success)", ls.enrichAttempts["tok-1"])
	}
}

func TestWorker_SkipsAlreadyEnriched(t *testing.T) {
	f := &mockItemFetcher{details: ItemDetails{Km: 99999}}
	ls := newMockListingStore()
	ls.enrichmentData["tok-1"] = storage.EnrichmentRecord{Km: 50000}
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != 0 {
		t.Errorf("expected 0 fetch calls for already-enriched token, got %d", f.calls)
	}
}

func TestWorker_SkipsWhenCityAlreadyEnriched(t *testing.T) {
	f := &mockItemFetcher{details: ItemDetails{Km: 0, City: "Tel Aviv"}}
	ls := newMockListingStore()
	ls.enrichmentData["tok-1"] = storage.EnrichmentRecord{Km: 0, City: "Tel Aviv"}
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != 0 {
		t.Errorf("expected 0 fetch calls when city is already enriched (even with Km=0), got %d", f.calls)
	}
}

func TestWorker_ChallengeTriggersBackoff(t *testing.T) {
	f := &mockItemFetcher{err: fetcher.ErrChallenge}
	ls := newMockListingStore()
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)

	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Errorf("expected ErrChallenge, got %v", err)
	}

	if d := rl.CurrentDelay(); d <= time.Millisecond {
		t.Errorf("delay should have increased after challenge, got %v", d)
	}

	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.enrichAttempts["tok-1"] != 1 {
		t.Errorf("enrich attempts = %d, want 1 (even on failure)", ls.enrichAttempts["tok-1"])
	}
}

func TestWorker_FetchErrorReturnsError(t *testing.T) {
	fetchErr := errors.New("network timeout")
	f := &mockItemFetcher{err: fetchErr}
	ls := newMockListingStore()
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)

	if err == nil {
		t.Error("expected error from fetch failure")
	}

	ls.mu.Lock()
	defer ls.mu.Unlock()
	if len(ls.backfilled) != 0 {
		t.Error("should not backfill on fetch error")
	}
}

func TestWorker_BackfillErrorReturnsError(t *testing.T) {
	f := &mockItemFetcher{details: ItemDetails{Km: 50000, City: "Haifa"}}
	ls := newMockListingStore()
	ls.backfillErr = errors.New("db write failed")
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)

	if err == nil {
		t.Error("expected error from backfill failure")
	}
}

func TestWorker_LookupErrorReturnsError(t *testing.T) {
	f := &mockItemFetcher{}
	ls := newMockListingStore()
	ls.lookupErr = errors.New("db read failed")
	rl := NewAdaptiveRateLimiter(time.Millisecond, time.Second, time.Second)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(context.Background(), req)

	if err == nil {
		t.Error("expected error from lookup failure")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != 0 {
		t.Error("should not fetch if lookup failed")
	}
}

func TestWorker_ContextCancellation(t *testing.T) {
	f := &mockItemFetcher{details: ItemDetails{Km: 50000}}
	ls := newMockListingStore()
	rl := NewAdaptiveRateLimiter(5*time.Second, 10*time.Second, time.Minute)
	w := NewWorker(f, ls, rl, testWorkerLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := broker.EnrichRequest{Token: "tok-1", Priority: 1, Source: "match"}
	err := w.HandleRequest(ctx, req)

	if err == nil {
		t.Error("expected error from cancelled context")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != 0 {
		t.Error("should not fetch when context is cancelled")
	}
}
