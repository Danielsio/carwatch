package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type prefillTrackingStore struct {
	lookupCalls atomic.Int32
}

func (s *prefillTrackingStore) LookupEnrichmentData(_ context.Context, _ []string) (map[string]storage.EnrichmentRecord, error) {
	s.lookupCalls.Add(1)
	return nil, nil
}

func (s *prefillTrackingStore) SaveListing(_ context.Context, _ storage.ListingRecord) error {
	return nil
}
func (s *prefillTrackingStore) SaveListings(_ context.Context, _ []storage.ListingRecord) error {
	return nil
}
func (s *prefillTrackingStore) BackfillListings(_ context.Context, _ []storage.ListingRecord) error {
	return nil
}
func (s *prefillTrackingStore) GetListing(_ context.Context, _ int64, _ string) (*storage.ListingRecord, error) {
	return nil, nil
}
func (s *prefillTrackingStore) ListUserListings(_ context.Context, _ int64, _, _ int) ([]storage.ListingRecord, error) {
	return nil, nil
}
func (s *prefillTrackingStore) CountUserListings(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *prefillTrackingStore) ListSearchListings(_ context.Context, _ int64, _ int64, _ storage.ListingFilter, _, _ int, _ string) ([]storage.ListingRecord, error) {
	return nil, nil
}
func (s *prefillTrackingStore) CountSearchListings(_ context.Context, _ int64, _ int64, _ storage.ListingFilter) (int64, error) {
	return 0, nil
}
func (s *prefillTrackingStore) CountSearchListingsForChat(_ context.Context, _ int64) (map[int64]int64, error) {
	return nil, nil
}
func (s *prefillTrackingStore) SearchStats(_ context.Context, _ int64, _ int64, _ storage.ListingFilter) (*storage.SearchStats, error) {
	return nil, nil
}
func (s *prefillTrackingStore) PruneListings(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}
func (s *prefillTrackingStore) DeleteStaleListings(_ context.Context, _ int64, _ int64, _ []string) (int64, error) {
	return 0, nil
}
func (s *prefillTrackingStore) DropListingByToken(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (s *prefillTrackingStore) ListUnenrichedTokens(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}
func (s *prefillTrackingStore) CountUnenrichedTokens(_ context.Context) (int64, error)   { return 0, nil }
func (s *prefillTrackingStore) IncrementEnrichAttempt(_ context.Context, _ string) error { return nil }
func (s *prefillTrackingStore) ResetUnenrichedAttempts(_ context.Context) (int64, error) {
	return 0, nil
}
func (s *prefillTrackingStore) ListEnrichExhaustedTokens(_ context.Context, _ []string, _ int) (map[string]bool, error) {
	return nil, nil
}
func (s *prefillTrackingStore) LookupListingIdentity(_ context.Context, _ string) (*storage.ListingIdentity, error) {
	return &storage.ListingIdentity{}, nil
}

func pipelineTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestPipeline_SkipPrefill(t *testing.T) {
	tracker := &prefillTrackingStore{}
	p := NewListingPipeline(tracker, nil, pipelineTestLogger())

	raw := []model.RawListing{
		{Token: "a", Price: 100000, Year: 2020, Manufacturer: "Mazda", Model: "3", Km: 0},
	}
	params := ProcessParams{
		ChatID:      100,
		SearchID:    1,
		SearchName:  "test",
		SkipPrefill: true,
		PriceMax:    150000,
	}

	p.Process(context.Background(), raw, params)

	if calls := tracker.lookupCalls.Load(); calls != 0 {
		t.Errorf("expected 0 LookupEnrichmentData calls with SkipPrefill=true, got %d", calls)
	}
}

func TestPipeline_WithPrefill(t *testing.T) {
	tracker := &prefillTrackingStore{}
	p := NewListingPipeline(tracker, nil, pipelineTestLogger())

	raw := []model.RawListing{
		{Token: "a", Price: 100000, Year: 2020, Manufacturer: "Mazda", Model: "3", Km: 0},
	}
	params := ProcessParams{
		ChatID:     100,
		SearchID:   1,
		SearchName: "test",
		PriceMax:   150000,
	}

	p.Process(context.Background(), raw, params)

	if calls := tracker.lookupCalls.Load(); calls != 1 {
		t.Errorf("expected 1 LookupEnrichmentData call with SkipPrefill=false, got %d", calls)
	}
}

type backfillTrackingStore struct {
	prefillTrackingStore
	backfillCalls   atomic.Int32
	backfillRecords []storage.ListingRecord
}

func (s *backfillTrackingStore) BackfillListings(_ context.Context, records []storage.ListingRecord) error {
	s.backfillCalls.Add(1)
	s.backfillRecords = append(s.backfillRecords, records...)
	return nil
}

func TestPipeline_InlineEnrichment_FillsMissingKm(t *testing.T) {
	store := &backfillTrackingStore{}
	p := NewListingPipeline(store, nil, pipelineTestLogger())
	p.SetInlineEnricher(func(_ context.Context, listings []model.RawListing) int {
		enriched := 0
		for i := range listings {
			if listings[i].Km <= 0 {
				listings[i].Km = 36900
				listings[i].City = "Tel Aviv"
				listings[i].ImageURL = "https://img.example.com/car.jpg"
				enriched++
			}
		}
		return enriched
	})

	raw := []model.RawListing{
		{Token: "t1", Price: 100000, Year: 2019, Manufacturer: "Honda", Model: "Civic", Km: 0},
	}
	params := ProcessParams{
		ChatID: 1, SearchID: 1, SearchName: "test",
		SkipPrefill: true, PriceMax: 150000,
	}

	result := p.Process(context.Background(), raw, params)

	if len(result.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(result.Records))
	}
	if result.Records[0].Km != 36900 {
		t.Errorf("expected km=36900 after inline enrichment, got %d", result.Records[0].Km)
	}
	if result.Records[0].City != "Tel Aviv" {
		t.Errorf("expected city=Tel Aviv, got %q", result.Records[0].City)
	}
	if store.backfillCalls.Load() != 1 {
		t.Errorf("expected 1 BackfillListings call, got %d", store.backfillCalls.Load())
	}
	if len(store.backfillRecords) != 1 || store.backfillRecords[0].Token != "t1" {
		t.Errorf("expected backfill for token t1, got %v", store.backfillRecords)
	}
}

func TestPipeline_InlineEnrichment_ScoresReflectEnrichedData(t *testing.T) {
	p := NewListingPipeline(nil, nil, pipelineTestLogger())

	raw := []model.RawListing{
		{Token: "t1", Price: 100000, Year: 2019, Manufacturer: "Honda", Model: "Civic", Km: 0, Hand: 2},
	}
	params := ProcessParams{
		ChatID: 1, SearchID: 1, SearchName: "test",
		SkipPrefill: true, PriceMax: 150000,
	}

	resultNoEnrich := p.Process(context.Background(), raw, params)
	scoreWithoutKm := *resultNoEnrich.Records[0].FitnessScore

	raw[0].Km = 0
	p.SetInlineEnricher(func(_ context.Context, listings []model.RawListing) int {
		for i := range listings {
			if listings[i].Km <= 0 {
				listings[i].Km = 36900
			}
		}
		return 1
	})
	resultEnriched := p.Process(context.Background(), raw, params)
	scoreWithKm := *resultEnriched.Records[0].FitnessScore

	if scoreWithKm <= scoreWithoutKm {
		t.Errorf("score with low km (%v) should be higher than without km (%v)", scoreWithKm, scoreWithoutKm)
	}
}

func TestPipeline_InlineEnrichment_NilEnricher_Skips(t *testing.T) {
	store := &backfillTrackingStore{}
	p := NewListingPipeline(store, nil, pipelineTestLogger())

	raw := []model.RawListing{
		{Token: "t1", Price: 100000, Year: 2019, Manufacturer: "Honda", Model: "Civic", Km: 0},
	}
	params := ProcessParams{
		ChatID: 1, SearchID: 1, SearchName: "test",
		SkipPrefill: true, PriceMax: 150000,
	}

	result := p.Process(context.Background(), raw, params)

	if result.Records[0].Km != 0 {
		t.Errorf("expected km=0 without enricher, got %d", result.Records[0].Km)
	}
	if store.backfillCalls.Load() != 0 {
		t.Errorf("expected 0 BackfillListings calls without enricher, got %d", store.backfillCalls.Load())
	}
}

func TestPipeline_InlineEnrichment_AllEnriched_NoBackfill(t *testing.T) {
	store := &backfillTrackingStore{}
	p := NewListingPipeline(store, nil, pipelineTestLogger())

	enricherCalled := false
	p.SetInlineEnricher(func(_ context.Context, _ []model.RawListing) int {
		enricherCalled = true
		return 0
	})

	raw := []model.RawListing{
		{Token: "t1", Price: 100000, Year: 2019, Manufacturer: "Honda", Model: "Civic",
			Km: 50000, City: "Haifa", ImageURL: "https://img.example.com/a.jpg"},
	}
	params := ProcessParams{
		ChatID: 1, SearchID: 1, SearchName: "test",
		SkipPrefill: true, PriceMax: 150000,
	}

	p.Process(context.Background(), raw, params)

	if enricherCalled {
		t.Error("enricher should not be called when all listings already have data")
	}
	if store.backfillCalls.Load() != 0 {
		t.Errorf("expected 0 BackfillListings calls, got %d", store.backfillCalls.Load())
	}
}

func TestPipeline_NilListingStore_NoPrefill(t *testing.T) {
	p := NewListingPipeline(nil, nil, pipelineTestLogger())

	raw := []model.RawListing{
		{Token: "b", Price: 80000, Year: 2019, Manufacturer: "Toyota", Model: "Corolla", Km: 50000},
	}
	params := ProcessParams{
		ChatID:     200,
		SearchID:   2,
		SearchName: "test2",
		PriceMax:   100000,
	}

	result := p.Process(context.Background(), raw, params)

	if len(result.Listings) != 1 {
		t.Errorf("expected 1 listing, got %d", len(result.Listings))
	}
}
