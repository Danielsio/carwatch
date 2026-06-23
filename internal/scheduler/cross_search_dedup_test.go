package scheduler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// perSearchDedup mirrors the real Postgres dedup behavior: the key includes
// searchID, so the same listing can be independently claimed for different
// searches belonging to the same user.
type perSearchDedup struct {
	mu   sync.Mutex
	seen map[perSearchDedupKey]bool
}

type perSearchDedupKey struct {
	token    string
	chatID   int64
	searchID int64
}

func newPerSearchDedup() *perSearchDedup {
	return &perSearchDedup{seen: make(map[perSearchDedupKey]bool)}
}

func (d *perSearchDedup) ClaimNew(_ context.Context, token string, chatID, searchID int64) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := perSearchDedupKey{token, chatID, searchID}
	if d.seen[key] {
		return false, nil
	}
	d.seen[key] = true
	return true, nil
}

func (d *perSearchDedup) ReleaseClaim(_ context.Context, token string, chatID, searchID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.seen, perSearchDedupKey{token, chatID, searchID})
	return nil
}

func (d *perSearchDedup) Prune(_ context.Context, _ time.Duration) (int64, error) { return 0, nil }
func (d *perSearchDedup) Close() error                                            { return nil }

// TestCrossSearchDedup_SameUserOverlappingSearches verifies that a listing
// matching multiple searches for the same user produces only one notification.
func TestCrossSearchDedup_SameUserOverlappingSearches(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-dup", ManufacturerID: 27, ModelID: 10332,
				Manufacturer: "Mazda", Model: "3",
				Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newPerSearchDedup()
	n := &mockNotifier{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "search-wide", Source: "yad2",
				Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2025, PriceMax: 200000, Active: true},
			{ID: 2, ChatID: 100, Name: "search-narrow", Source: "yad2",
				Manufacturer: 27, Model: 10332,
				YearMin: 2019, YearMax: 2022, PriceMax: 150000, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{SearchStore: ss})

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification (cross-search dedup), got %d", len(n.messages))
	}
}

// TestCrossSearchDedup_DifferentUsers verifies that cross-search dedup only
// applies within the same user — different users still get their own alerts.
func TestCrossSearchDedup_DifferentUsers(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-shared", ManufacturerID: 27, ModelID: 10332,
				Manufacturer: "Mazda", Model: "3",
				Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newPerSearchDedup()
	n := &mockNotifier{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1-search", Source: "yad2",
				Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2025, PriceMax: 200000, Active: true},
			{ID: 2, ChatID: 200, Name: "user2-search", Source: "yad2",
				Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2025, PriceMax: 200000, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{SearchStore: ss})

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 2 {
		t.Errorf("expected 2 notifications (one per user), got %d", len(n.messages))
	}
}

// TestCrossSearchDedup_PriceDrop verifies that a price drop matching multiple
// searches for the same user only sends one price drop notification.
func TestCrossSearchDedup_PriceDrop(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-pd", ManufacturerID: 27, ModelID: 10332,
				Manufacturer: "Mazda", Model: "3",
				Price: 85000, Year: 2020, EngineVolume: 2000, Km: 50000},
		},
	}
	d := newPerSearchDedup()
	n := &mockNotifier{}

	pt := newMockPriceTracker()
	pt.prices["tok-pd"] = 95000

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "search-a", Source: "yad2",
				Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2025, PriceMax: 200000, Active: true},
			{ID: 2, ChatID: 100, Name: "search-b", Source: "yad2",
				Manufacturer: 27, Model: 10332,
				YearMin: 2019, YearMax: 2022, PriceMax: 150000, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		Prices:      pt,
	})

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.messages) != 0 {
		t.Errorf("expected 0 batch notifications for price drop, got %d", len(n.messages))
	}
	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 price drop notification (cross-search dedup), got %d", len(n.rawMessages))
	}
	if len(n.rawMessages) > 0 {
		msg := n.rawMessages[0].message
		if !strings.Contains(msg, "95,000") || !strings.Contains(msg, "85,000") {
			t.Errorf("price drop message should contain old and new prices, got:\n%s", msg)
		}
	}
}
