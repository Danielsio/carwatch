package scheduler

import (
	"context"
	"testing"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func TestPostEnrichmentKmFilter_ExceedsLimit(t *testing.T) {
	// Listing arrives with km already known (e.g. from prefill or enricher
	// worker). The scheduler's post-enrichment MaxKm filter should drop it
	// and release the dedup claim.
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "high-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 150000, City: "Haifa", ImageURL: "https://img.com/1.jpg"},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 100000, Active: true},
		},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 0 {
		t.Errorf("expected 0 notifications (km exceeded MaxKm), got %d", len(n.messages))
	}

	// Verify the dedup claim was released so the listing can be re-evaluated.
	d.mu.Lock()
	defer d.mu.Unlock()
	key := dedupKey{token: "high-km", chatID: 100}
	if d.seen[key] {
		t.Error("dedup claim should be released for km-filtered listing")
	}
}

func TestPostEnrichmentKmFilter_WithinLimit(t *testing.T) {
	// Listing arrives with km already known and within the search's MaxKm cap.
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "good-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 80000, City: "Tel Aviv", ImageURL: "https://img.com/1.jpg"},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 100000, Active: true},
		},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification (km within limit), got %d", len(n.messages))
	}
}

func TestNoKmFilter_ListingDeliveredWithoutKm(t *testing.T) {
	// When no search has a MaxKm filter, listings are delivered even without km.
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "no-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "all-mazda", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 0, Active: true},
		},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification (listing should be delivered without km filter), got %d", len(n.messages))
	}
}
