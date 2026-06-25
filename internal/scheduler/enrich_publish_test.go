package scheduler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/redis/go-redis/v9"
)

func newTestEnrichPublisher(t *testing.T) (*broker.EnrichPublisher, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return broker.NewEnrichPublisher(client), s
}

func readEnrichRequests(t *testing.T, s *miniredis.Miniredis) []broker.EnrichRequest {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	msgs, err := client.XRange(context.Background(), broker.EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}

	var reqs []broker.EnrichRequest
	for _, msg := range msgs {
		data, ok := msg.Values["data"].(string)
		if !ok {
			t.Fatal("expected data field as string")
		}
		var req broker.EnrichRequest
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		reqs = append(reqs, req)
	}
	return reqs
}

func TestDualWrite_MatchedListingsPublished(t *testing.T) {
	enrichPub, redisSrv := newTestEnrichPublisher(t)

	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-1", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 90000, Year: 2020, EngineVolume: 2000, Km: 0, City: "", ImageURL: ""},
			{Token: "tok-2", ManufacturerID: 99, Manufacturer: "BMW", ModelID: 555, Model: "X5",
				Price: 200000, Year: 2022, EngineVolume: 3000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024,
				PriceMax: 150000, Active: true},
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

	reqs := readEnrichRequests(t, redisSrv)

	// Only tok-1 should be published (matched and needs enrichment: Km=0, City="", ImageURL="").
	// tok-2 doesn't match the search.
	if len(reqs) != 1 {
		t.Fatalf("expected 1 enrich request, got %d", len(reqs))
	}
	if reqs[0].Token != "tok-1" {
		t.Errorf("token = %q, want tok-1", reqs[0].Token)
	}
	if reqs[0].Priority != 1 {
		t.Errorf("priority = %d, want 1 (matched listing)", reqs[0].Priority)
	}
	if reqs[0].Source != "scheduler" {
		t.Errorf("source = %q, want scheduler", reqs[0].Source)
	}
	if len(reqs[0].SearchIDs) != 1 || reqs[0].SearchIDs[0] != 1 {
		t.Errorf("search_ids = %v, want [1]", reqs[0].SearchIDs)
	}
}

func TestDualWrite_NoPublishWhenAlreadyEnriched(t *testing.T) {
	enrichPub, redisSrv := newTestEnrichPublisher(t)

	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-enriched", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 90000, Year: 2020, EngineVolume: 2000, Km: 50000, City: "Tel Aviv", ImageURL: "https://img.com/1.jpg"},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024,
				PriceMax: 150000, Active: true},
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

	reqs := readEnrichRequests(t, redisSrv)

	// Listing already has all enrichment data, so nothing should be published.
	if len(reqs) != 0 {
		t.Errorf("expected 0 enrich requests for already-enriched listing, got %d", len(reqs))
	}
}

func TestDualWrite_NilPublisherNoOp(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-1", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 90000, Year: 2020, Km: 0},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024,
				PriceMax: 150000, Active: true},
		},
	}

	// No EnrichPublisher — should work fine without publishing.
	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle should succeed without enrich publisher: %v", err)
	}

	// Listing should still be delivered.
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification, got %d", len(n.messages))
	}
}

func TestBackfillPublishesAtPriority3(t *testing.T) {
	enrichPub, redisSrv := newTestEnrichPublisher(t)

	ls := &mockListingStore{
		unenrichedTokens: []string{"old-1", "old-2", "old-3"},
	}

	cfg := testConfig()
	s, err := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{
		SearchStore:     &mockSearchStore{},
		ListingStore:    ls,
		EnrichPublisher: enrichPub,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	s.backfillUnenrichedListings(context.Background())

	reqs := readEnrichRequests(t, redisSrv)

	if len(reqs) != 3 {
		t.Fatalf("expected 3 backfill enrich requests, got %d", len(reqs))
	}

	for _, req := range reqs {
		if req.Priority != 3 {
			t.Errorf("backfill priority = %d, want 3", req.Priority)
		}
		if req.Source != "backfill" {
			t.Errorf("source = %q, want backfill", req.Source)
		}
	}
}

func TestDualWrite_MultipleSearchesSharePublish(t *testing.T) {
	enrichPub, redisSrv := newTestEnrichPublisher(t)

	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-shared", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 100000, Year: 2020, EngineVolume: 2000, Km: 0},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1", Source: "yad2",
				Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024,
				PriceMax: 200000, Active: true},
			{ID: 2, ChatID: 200, Name: "user2", Source: "yad2",
				Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2026,
				PriceMax: 150000, Active: true},
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

	reqs := readEnrichRequests(t, redisSrv)

	// One listing matched by two searches should produce one enrich request
	// (since publishing is per matched listing index, not per search).
	if len(reqs) != 1 {
		t.Fatalf("expected 1 enrich request for shared listing, got %d", len(reqs))
	}

	if reqs[0].Token != "tok-shared" {
		t.Errorf("token = %q, want tok-shared", reqs[0].Token)
	}
	// Both search IDs should be in the request.
	if len(reqs[0].SearchIDs) != 2 {
		t.Errorf("expected 2 search IDs, got %d: %v", len(reqs[0].SearchIDs), reqs[0].SearchIDs)
	}
}

func TestDualWrite_ExhaustedTokenNotPublished(t *testing.T) {
	enrichPub, redisSrv := newTestEnrichPublisher(t)

	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tok-ok", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 90000, Year: 2020, EngineVolume: 2000, Km: 0, City: "", ImageURL: ""},
			{Token: "tok-exhausted", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 95000, Year: 2021, EngineVolume: 2000, Km: 0, City: "", ImageURL: ""},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024,
				PriceMax: 150000, Active: true},
		},
	}

	ls := &mockListingStore{
		exhaustedTokens: map[string]bool{"tok-exhausted": true},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore:     ss,
		ListingStore:    ls,
		EnrichPublisher: enrichPub,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	reqs := readEnrichRequests(t, redisSrv)

	// Only tok-ok should be published; tok-exhausted has enrich_attempts >= 10.
	if len(reqs) != 1 {
		t.Fatalf("expected 1 enrich request (tok-exhausted skipped), got %d", len(reqs))
	}
	if reqs[0].Token != "tok-ok" {
		t.Errorf("token = %q, want tok-ok", reqs[0].Token)
	}
}
