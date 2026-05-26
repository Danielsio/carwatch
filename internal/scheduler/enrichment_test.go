package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type trackingEnricher struct {
	mu       sync.Mutex
	calls    int
	tokens   [][]string
	enrichFn func(listings []model.RawListing) int
}

func (e *trackingEnricher) Enrich(_ context.Context, listings []model.RawListing) int {
	e.mu.Lock()
	tokens := make([]string, len(listings))
	for i, l := range listings {
		tokens[i] = l.Token
	}
	e.tokens = append(e.tokens, tokens)
	e.calls++
	e.mu.Unlock()
	if e.enrichFn != nil {
		return e.enrichFn(listings)
	}
	return 0
}

func TestMatchFirstEnrichment_OnlyEnrichesMatchedListings(t *testing.T) {
	enricher := &trackingEnricher{
		enrichFn: func(listings []model.RawListing) int {
			for i := range listings {
				listings[i].Km = 50000
			}
			return len(listings)
		},
	}

	f := &mockFetcher{listings: []model.RawListing{
		{Token: "match-1", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3", Price: 90000, Year: 2020},
		{Token: "match-2", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3", Price: 85000, Year: 2021},
		{Token: "no-match-1", ManufacturerID: 99, Manufacturer: "BMW", ModelID: 555, Model: "X5", Price: 200000, Year: 2022},
		{Token: "no-match-2", ManufacturerID: 88, Manufacturer: "Audi", ModelID: 444, Model: "A4", Price: 150000, Year: 2020},
		{Token: "no-match-3", ManufacturerID: 77, Manufacturer: "Honda", ModelID: 333, Model: "Civic", Price: 70000, Year: 2019},
	}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "mazda3", Source: "yad2",
				Manufacturer: 27, Model: 10332, MaxKm: 200000, Active: true},
		},
	}

	s, err := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		KmEnricher:  enricher,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	enricher.mu.Lock()
	defer enricher.mu.Unlock()

	if enricher.calls != 1 {
		t.Fatalf("expected 1 enricher call, got %d", enricher.calls)
	}

	enrichedTokens := enricher.tokens[0]
	tokenSet := make(map[string]bool)
	for _, tok := range enrichedTokens {
		tokenSet[tok] = true
	}

	if !tokenSet["match-1"] || !tokenSet["match-2"] {
		t.Errorf("expected matched tokens to be enriched, got %v", enrichedTokens)
	}
	if tokenSet["no-match-1"] || tokenSet["no-match-2"] || tokenSet["no-match-3"] {
		t.Errorf("unmatched tokens should not be enriched, got %v", enrichedTokens)
	}
	if len(enrichedTokens) != 2 {
		t.Errorf("expected 2 tokens enriched, got %d: %v", len(enrichedTokens), enrichedTokens)
	}
}

func TestMatchFirstEnrichment_PostEnrichmentKmFilter(t *testing.T) {
	enricher := &trackingEnricher{
		enrichFn: func(listings []model.RawListing) int {
			for i := range listings {
				listings[i].Km = 150000
			}
			return len(listings)
		},
	}

	f := &mockFetcher{listings: []model.RawListing{
		{Token: "high-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000},
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
		KmEnricher:  enricher,
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
		t.Errorf("expected 0 notifications (km exceeded MaxKm after enrichment), got %d", len(n.messages))
	}

	// Verify the dedup claim was released so the listing can be re-evaluated.
	d.mu.Lock()
	defer d.mu.Unlock()
	key := dedupKey{token: "high-km", chatID: 100}
	if d.seen[key] {
		t.Error("dedup claim should be released for km-filtered listing")
	}
}

func TestMatchFirstEnrichment_KmWithinLimit(t *testing.T) {
	enricher := &trackingEnricher{
		enrichFn: func(listings []model.RawListing) int {
			for i := range listings {
				listings[i].Km = 80000
				listings[i].City = "Tel Aviv"
			}
			return len(listings)
		},
	}

	f := &mockFetcher{listings: []model.RawListing{
		{Token: "good-km", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
			Price: 90000, Year: 2020, EngineVolume: 2000},
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
		KmEnricher:  enricher,
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

func TestMatchFirstEnrichment_SkipsEnrichmentWhenNoKmFilter(t *testing.T) {
	enricher := &trackingEnricher{}

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
		KmEnricher:  enricher,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := s.runMultiTenantCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	enricher.mu.Lock()
	defer enricher.mu.Unlock()
	if enricher.calls != 0 {
		t.Errorf("expected 0 enricher calls when no search has MaxKm filter, got %d", enricher.calls)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification (listing should still be delivered without enrichment), got %d", len(n.messages))
	}
}

func TestBackfillBatchSize(t *testing.T) {
	var capturedLimit int
	ls := &batchTrackingListingStore{
		mockListingStore: mockListingStore{
			unenrichedTokens: []string{"t1", "t2", "t3"},
		},
		captureLimit: &capturedLimit,
	}

	enrichCalls := 0
	enricher := &countingEnricher{calls: &enrichCalls}

	cfg := testConfig()
	s, err := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{
		SearchStore:      &mockSearchStore{},
		ListingStore:     ls,
		KmEnricher:       enricher,
		BackfillCooldown: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	s.backfillUnenrichedListings(context.Background())

	if capturedLimit != 15 {
		t.Errorf("expected backfill batch limit of 15, got %d", capturedLimit)
	}
}

type batchTrackingListingStore struct {
	mockListingStore
	captureLimit *int
}

func (m *batchTrackingListingStore) ListUnenrichedTokens(_ context.Context, limit int) ([]string, error) {
	*m.captureLimit = limit
	return m.unenrichedTokens, nil
}
