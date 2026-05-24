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
func (s *prefillTrackingStore) ListUnenrichedTokens(_ context.Context, _ int) ([]string, error) {
	return nil, nil
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
