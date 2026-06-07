package scheduler

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/redis/go-redis/v9"
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

func TestMultiSearchKmDivergence(t *testing.T) {
	// Same listing (token="shared") matches search A (MaxKm=50000) and search B
	// (MaxKm=150000). Listing has Km=100000 pre-set (simulating prefill).
	// Search A should filter it out; search B should deliver it.
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "shared", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 100000, City: "Haifa", ImageURL: "https://img.com/1.jpg"},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "strict-km", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 50000, Active: true},
			{ID: 2, ChatID: 200, Name: "relaxed-km", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 150000, Active: true},
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

	// Only chatID 200 (MaxKm=150000) should receive a notification.
	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notification (only relaxed-km search), got %d", len(n.messages))
	}
	if n.messages[0].recipient != "200" {
		t.Errorf("expected notification for chatID 200, got %q", n.messages[0].recipient)
	}
}

func TestPostEnrichmentKmFilter_ReleasesDedup(t *testing.T) {
	// Listing with Km=150000 matches search with MaxKm=100000.
	// After the cycle, the dedup claim should be released.
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "over-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 150000, City: "Haifa", ImageURL: "https://img.com/1.jpg"},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "strict-km", Source: "yad2",
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

	// Verify no notification was sent.
	n.mu.Lock()
	if len(n.messages) != 0 {
		t.Errorf("expected 0 notifications (km exceeds limit), got %d", len(n.messages))
	}
	n.mu.Unlock()

	// Verify the dedup claim was released so the listing can be re-evaluated.
	d.mu.Lock()
	defer d.mu.Unlock()
	key := dedupKey{token: "over-km", chatID: 100}
	if d.seen[key] {
		t.Error("dedup claim should be released for km-filtered listing")
	}
}

func TestNoKmFilter_SkipsEnrichmentPublish(t *testing.T) {
	// Listing matches search with MaxKm=0 (no km filter).
	// The listing should be delivered but no enrichment request should be
	// published to Redis (listing already has km/city/image, so the
	// enrichment condition is not triggered).
	redisSrv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisSrv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	enrichPub := broker.NewEnrichPublisher(client)

	f := &mockFetcher{listings: []model.RawListing{
		{Token: "complete", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 80000, City: "Tel Aviv", ImageURL: "https://img.com/1.jpg"},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "no-km-limit", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 0, Active: true},
		},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore:     ss,
		EnrichPublisher: enrichPub,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	// Listing should be delivered.
	n.mu.Lock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification, got %d", len(n.messages))
	}
	n.mu.Unlock()

	// No enrichment requests should be published (listing already has km/city/image).
	reqs := readEnrichRequests(t, redisSrv)
	if len(reqs) != 0 {
		t.Errorf("expected 0 enrichment requests (listing already complete), got %d", len(reqs))
	}
}

func TestEmptyFeed_NoEnrichment(t *testing.T) {
	// Fetcher returns empty slice with an active search.
	// No enrichment requests should be published, cycle should complete without error.
	redisSrv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisSrv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	enrichPub := broker.NewEnrichPublisher(client)

	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "active-search", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 100000, Active: true},
		},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore:     ss,
		EnrichPublisher: enrichPub,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle should complete without error for empty feed: %v", err)
	}

	// No notifications.
	n.mu.Lock()
	if len(n.messages) != 0 {
		t.Errorf("expected 0 notifications for empty feed, got %d", len(n.messages))
	}
	n.mu.Unlock()

	// No enrichment requests published.
	reqs := readEnrichRequests(t, redisSrv)
	if len(reqs) != 0 {
		t.Errorf("expected 0 enrichment requests for empty feed, got %d", len(reqs))
	}
}

func TestUnenrichedListingHeldBack_WhenMaxKmSet(t *testing.T) {
	// Listing arrives with Km=0 (not yet enriched). Search has MaxKm=130000.
	// The listing should be held back (0 notifications) and the dedup claim
	// released so it can be re-evaluated once the enricher fills in km.
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "pending-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 0, City: "", ImageURL: ""},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3-km-cap", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 130000, Active: true},
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
		t.Errorf("expected 0 notifications (km unknown, MaxKm set), got %d", len(n.messages))
	}

	// Verify the dedup claim was released so the listing re-matches after enrichment.
	d.mu.Lock()
	defer d.mu.Unlock()
	key := dedupKey{token: "pending-km", chatID: 100}
	if d.seen[key] {
		t.Error("dedup claim should be released for unenriched listing when MaxKm is set")
	}
}

func TestPartialEnrichment(t *testing.T) {
	// 3 listings match a search. Two have km data (simulating partial
	// enrichment), the third has Km=0. All 3 should be delivered since the
	// search has no MaxKm filter (MaxKm=0).
	f := &mockFetcher{listings: []model.RawListing{
		{Token: "enriched-1", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000, Km: 50000, City: "Haifa", ImageURL: "https://img.com/1.jpg"},
		{Token: "enriched-2", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 85000, Year: 2019, EngineVolume: 2000, Km: 70000, City: "Tel Aviv", ImageURL: "https://img.com/2.jpg"},
		{Token: "unenriched", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 80000, Year: 2021, EngineVolume: 2000, Km: 0, City: "", ImageURL: ""},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "all-mazda3", Source: "yad2",
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

	// All 3 listings should be delivered (MaxKm=0 means no km filtering).
	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notification call, got %d", len(n.messages))
	}
	if n.messages[0].count != 3 {
		t.Errorf("expected 3 listings in notification (including unenriched), got %d", n.messages[0].count)
	}
}
