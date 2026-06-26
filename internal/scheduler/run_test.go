package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func TestRun_ContextCancel(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()
	cfg.Polling.Interval = 1 * time.Second
	cfg.Polling.Jitter = 0

	ss := &mockSearchStore{}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = s.Run(ctx)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestRun_OutsideActiveHours(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()
	cfg.Polling.Interval = 1 * time.Second
	cfg.Polling.Jitter = 0
	cfg.Polling.ActiveHours = &config.ActiveHours{
		Start: "00:00",
		End:   "00:01",
	}

	ss := &mockSearchStore{}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = s.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestRunMultiTenantCycle_AllGroupsFail(t *testing.T) {
	f := &mockFetcher{err: context.DeadlineExceeded}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()
	h := health.New()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		Observer:    h,
	})

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Errorf("targeted-only cycle should not return error (individual fetch errors are logged and skipped): %v", err)
	}
}

func TestRunMultiTenantCycle_PrunesOldListings(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "a", ManufacturerID: 27, ModelID: 10332, Price: 100000, Year: 2020},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()
	cfg.Storage.PruneAfter = 1 * time.Hour

	ls := &mockListingStore{}
	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332, Active: true},
		},
	}
	h := health.New()

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore:  ss,
		ListingStore: ls,
		Observer:     h,
	})
	s.lastPruneTime = time.Time{}

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if s.lastPruneTime.IsZero() {
		t.Error("lastPruneTime should be updated after pruning")
	}
	if ls.pruneCalls != 1 {
		t.Errorf("expected PruneListings to be called once, got %d", ls.pruneCalls)
	}
}

func TestProcessGroup_NotifyFails_ReleaseClaims(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{err: context.DeadlineExceeded}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{SearchStore: ss})

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	d.mu.Lock()
	_, claimed := d.seen[dedupKey{"a", 100}]
	d.mu.Unlock()
	if claimed {
		t.Error("claim should be released after notification failure")
	}
}

func TestProcessGroup_SavesListings(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ls := &mockListingStore{}
	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore:  ss,
		ListingStore: ls,
	})

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	if len(ls.saved) != 1 {
		t.Errorf("expected 1 saved listing, got %d", len(ls.saved))
	}
}

func TestFlushAndSendDigest(t *testing.T) {
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()
	ds.items[100] = digestItems("item1", "item2", "item3")

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DigestStore: ds})

	s.flushAndSendDigest(context.Background(), 100)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Fatalf("expected 1 digest message, got %d", len(n.rawMessages))
	}
	if n.rawMessages[0].recipient != "100" {
		t.Errorf("recipient = %q, want 100", n.rawMessages[0].recipient)
	}
}

func TestFlushAndSendDigest_Empty(t *testing.T) {
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DigestStore: ds})

	s.flushAndSendDigest(context.Background(), 100)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 0 {
		t.Errorf("expected 0 messages for empty digest, got %d", len(n.rawMessages))
	}
}

func TestFlushAndSendDigest_WithHealth(t *testing.T) {
	n := &mockNotifier{}
	cfg := testConfig()
	h := health.New()

	ds := newMockDigestStore()
	ds.items[100] = digestItems("item1")

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{
		DigestStore: ds,
		Observer:    h,
	})

	s.flushAndSendDigest(context.Background(), 100)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 message, got %d", len(n.rawMessages))
	}
}

type mockListingStore struct {
	saved            []storage.ListingRecord
	pruneCalls       int
	unenrichedTokens []string
	exhaustedTokens  map[string]bool
}

func (m *mockListingStore) SaveListing(_ context.Context, r storage.ListingRecord) error {
	m.saved = append(m.saved, r)
	return nil
}

func (m *mockListingStore) SaveListings(_ context.Context, records []storage.ListingRecord) error {
	m.saved = append(m.saved, records...)
	return nil
}

func (m *mockListingStore) BackfillListings(_ context.Context, _ []storage.ListingRecord) error {
	return nil
}

func (m *mockListingStore) LookupEnrichmentData(_ context.Context, _ []string) (map[string]storage.EnrichmentRecord, error) {
	return nil, nil
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

func (m *mockListingStore) PruneListings(_ context.Context, _ time.Duration) (int64, error) {
	m.pruneCalls++
	return 0, nil
}

func (m *mockListingStore) SearchStats(_ context.Context, _ int64, _ int64, _ storage.ListingFilter) (*storage.SearchStats, error) {
	return &storage.SearchStats{}, nil
}

func (m *mockListingStore) DeleteStaleListings(_ context.Context, _ int64, _ int64, _ []string) (int64, error) {
	return 0, nil
}
func (m *mockListingStore) DropListingByToken(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *mockListingStore) ListUnenrichedTokens(_ context.Context, _ int) ([]string, error) {
	return m.unenrichedTokens, nil
}
func (m *mockListingStore) CountUnenrichedTokens(_ context.Context) (int64, error)   { return 0, nil }
func (m *mockListingStore) IncrementEnrichAttempt(_ context.Context, _ string) error { return nil }
func (m *mockListingStore) ResetUnenrichedAttempts(_ context.Context) (int64, error) {
	return 0, nil
}
func (m *mockListingStore) ListEnrichExhaustedTokens(_ context.Context, tokens []string, _ int) (map[string]bool, error) {
	if m.exhaustedTokens == nil {
		return nil, nil
	}
	result := make(map[string]bool)
	for _, tok := range tokens {
		if m.exhaustedTokens[tok] {
			result[tok] = true
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}
func (m *mockListingStore) LookupListingIdentity(_ context.Context, _ string) (*storage.ListingIdentity, error) {
	return &storage.ListingIdentity{}, nil
}
