package scheduler

import (
	"context"
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
	if err == nil || err != context.DeadlineExceeded {
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
	if err != context.DeadlineExceeded {
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
	if err == nil {
		t.Error("expected error when all groups fail")
	}
}

func TestRunMultiTenantCycle_PrunesOldListings(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()
	cfg.Storage.PruneAfter = 1 * time.Hour

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332, Active: true},
		},
	}
	h := health.New()

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		Observer:    h,
	})
	s.lastPruneTime = time.Time{}

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if s.lastPruneTime.IsZero() {
		t.Error("lastPruneTime should be updated after pruning")
	}
}

func TestProcessGroup_NotifyFails_ReleaseClaims(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
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
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
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
	ds.items[100] = []string{"item1", "item2", "item3"}

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
	ds.items[100] = []string{"item1"}

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
	saved []storage.ListingRecord
}

func (m *mockListingStore) SaveListing(_ context.Context, r storage.ListingRecord) error {
	m.saved = append(m.saved, r)
	return nil
}

func (m *mockListingStore) ListUserListings(_ context.Context, _ int64, _, _ int) ([]storage.ListingRecord, error) {
	return nil, nil
}

func (m *mockListingStore) CountUserListings(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
