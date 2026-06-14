package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// paramAwareFetcher returns different listings based on the full SourceParams.
type paramAwareFetcher struct {
	mu       sync.Mutex
	calls    []model.SourceParams
	results  map[string][]model.RawListing
	errors   map[string]error
	fallback []model.RawListing
}

func paramKeyStr(p model.SourceParams) string {
	return fmt.Sprintf("%d:%d:%d-%d:%d-%d:%d:%d:%d",
		p.Manufacturer, p.Model, p.YearMin, p.YearMax,
		p.PriceMin, p.PriceMax, p.MaxKm, p.MaxHand, p.EngineMinCC)
}

func (f *paramAwareFetcher) Fetch(_ context.Context, p model.SourceParams) ([]model.RawListing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, p)
	key := paramKeyStr(p)
	if err, ok := f.errors[key]; ok {
		return nil, err
	}
	if res, ok := f.results[key]; ok {
		return res, nil
	}
	return f.fallback, nil
}

func newTestSchedulerWithFetcher(f fetcher.Fetcher) *Scheduler {
	cfg := testConfig()
	s, err := NewWithOptions(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), Options{})
	if err != nil {
		panic(err)
	}
	return s
}

func TestFetchTargetedListings_NoPairsNoFetch(t *testing.T) {
	f := &paramAwareFetcher{}
	s := newTestSchedulerWithFetcher(f)

	// Wildcard searches (Manufacturer=0 or Model=0) should not trigger targeted fetches.
	searches := []storage.Search{
		{ID: 1, Manufacturer: 0, Model: 0},
		{ID: 2, Manufacturer: 27, Model: 0},
	}
	global := []model.RawListing{{Token: "g1"}}
	result, _ := s.fetchTargetedListings(context.Background(), searches, global, f)

	if len(result) != 1 {
		t.Errorf("expected 1 listing (unchanged), got %d", len(result))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("expected 0 fetch calls for wildcard searches, got %d", len(f.calls))
	}
}

func TestFetchTargetedListings_FetchesWithFullParams(t *testing.T) {
	// Search has year/price/km filters — targeted fetch should pass them all.
	sr := storage.Search{
		ID: 1, Manufacturer: 27, Model: 10332,
		YearMin: 2020, YearMax: 2024, PriceMax: 150000,
		MaxKm: 130000, MaxHand: 3, EngineMinCC: 1600,
	}
	key := paramKeyStr(model.SourceParamsFromSearch(&sr))

	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key: {
				{Token: "t1", ManufacturerID: 27, ModelID: 10332},
				{Token: "t2", ManufacturerID: 27, ModelID: 10332},
			},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	global := []model.RawListing{{Token: "g1", ManufacturerID: 99, ModelID: 999}}
	result, _ := s.fetchTargetedListings(context.Background(), []storage.Search{sr}, global, f)

	if len(result) != 3 {
		t.Errorf("expected 3 listings (1 global + 2 targeted), got %d", len(result))
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 targeted fetch call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if c.Manufacturer != 27 || c.Model != 10332 {
		t.Errorf("wrong manufacturer/model: %+v", c)
	}
	if c.YearMin != 2020 || c.YearMax != 2024 {
		t.Errorf("expected year range 2020-2024, got %d-%d", c.YearMin, c.YearMax)
	}
	if c.PriceMax != 150000 {
		t.Errorf("expected PriceMax=150000, got %d", c.PriceMax)
	}
	if c.MaxKm != 130000 {
		t.Errorf("expected MaxKm=130000, got %d", c.MaxKm)
	}
}

func TestFetchTargetedListings_AlwaysFetchesRegardlessOfCoverage(t *testing.T) {
	// Even with many global-feed listings for a model, a targeted fetch
	// should still run to ensure full filter coverage.
	sr := storage.Search{ID: 1, Manufacturer: 27, Model: 10332, YearMin: 2020}
	key := paramKeyStr(model.SourceParamsFromSearch(&sr))
	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key: {{Token: "targeted1", ManufacturerID: 27, ModelID: 10332}},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	// Global feed already has 10 listings for this model.
	global := make([]model.RawListing, 10)
	for i := range global {
		global[i] = model.RawListing{
			Token:          fmt.Sprintf("g%d", i),
			ManufacturerID: 27,
			ModelID:        10332,
		}
	}

	result, _ := s.fetchTargetedListings(context.Background(), []storage.Search{sr}, global, f)

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 {
		t.Errorf("expected 1 fetch call (no coverage threshold), got %d", len(f.calls))
	}
	if len(result) != 11 {
		t.Errorf("expected 11 listings (10 global + 1 targeted), got %d", len(result))
	}
}

func TestFetchTargetedListings_DeduplicatesTokens(t *testing.T) {
	sr := storage.Search{ID: 1, Manufacturer: 17, Model: 10182}
	key := paramKeyStr(model.SourceParamsFromSearch(&sr))
	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key: {
				{Token: "shared", ManufacturerID: 17, ModelID: 10182},
				{Token: "new1", ManufacturerID: 17, ModelID: 10182},
			},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	global := []model.RawListing{{Token: "shared", ManufacturerID: 17, ModelID: 10182}}
	result, _ := s.fetchTargetedListings(context.Background(), []storage.Search{sr}, global, f)

	if len(result) != 2 {
		t.Errorf("expected 2 listings (1 shared deduped + 1 new), got %d", len(result))
	}
	tokens := make(map[string]int)
	for _, l := range result {
		tokens[l.Token]++
	}
	if tokens["shared"] != 1 {
		t.Errorf("token 'shared' appeared %d times, want 1", tokens["shared"])
	}
	if tokens["new1"] != 1 {
		t.Errorf("token 'new1' appeared %d times, want 1", tokens["new1"])
	}
}

func TestFetchTargetedListings_FetchErrorContinues(t *testing.T) {
	sr1 := storage.Search{ID: 1, Manufacturer: 27, Model: 10332}
	sr2 := storage.Search{ID: 2, Manufacturer: 19, Model: 10222}
	key2 := paramKeyStr(model.SourceParamsFromSearch(&sr2))

	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key2: {{Token: "ok1", ManufacturerID: 19, ModelID: 10222}},
		},
		errors: map[string]error{
			paramKeyStr(model.SourceParamsFromSearch(&sr1)): fetcher.ErrChallenge,
		},
	}
	s := newTestSchedulerWithFetcher(f)

	global := []model.RawListing{{Token: "g1"}}
	result, _ := s.fetchTargetedListings(context.Background(), []storage.Search{sr1, sr2}, global, f)

	hasOk1 := false
	for _, l := range result {
		if l.Token == "ok1" {
			hasOk1 = true
		}
	}
	if !hasOk1 {
		t.Error("expected targeted listing 'ok1' from successful search, not found")
	}
}

func TestFetchTargetedListings_IdenticalParamsFetchedOnce(t *testing.T) {
	// Two searches with the same model AND same filters → one fetch.
	sr1 := storage.Search{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332, YearMin: 2020}
	sr2 := storage.Search{ID: 2, ChatID: 200, Manufacturer: 27, Model: 10332, YearMin: 2020}
	key := paramKeyStr(model.SourceParamsFromSearch(&sr1))

	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key: {{Token: "t1", ManufacturerID: 27, ModelID: 10332}},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	global := []model.RawListing{{Token: "g1"}}
	result, _ := s.fetchTargetedListings(context.Background(), []storage.Search{sr1, sr2}, global, f)

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 {
		t.Errorf("expected 1 targeted fetch for identical params, got %d", len(f.calls))
	}
	if len(result) != 2 {
		t.Errorf("expected 2 listings (1 global + 1 targeted), got %d", len(result))
	}
}

func TestFetchTargetedListings_DifferentFiltersFetchSeparately(t *testing.T) {
	// Two searches for same model with different year ranges → two fetches.
	sr1 := storage.Search{ID: 1, Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2020}
	sr2 := storage.Search{ID: 2, Manufacturer: 27, Model: 10332, YearMin: 2021, YearMax: 2024}
	key1 := paramKeyStr(model.SourceParamsFromSearch(&sr1))
	key2 := paramKeyStr(model.SourceParamsFromSearch(&sr2))

	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key1: {{Token: "old1", ManufacturerID: 27, ModelID: 10332}},
			key2: {{Token: "new1", ManufacturerID: 27, ModelID: 10332}},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	global := []model.RawListing{{Token: "g1"}}
	result, _ := s.fetchTargetedListings(context.Background(), []storage.Search{sr1, sr2}, global, f)

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 2 {
		t.Errorf("expected 2 targeted fetches for different filters, got %d", len(f.calls))
	}
	if len(result) != 3 {
		t.Errorf("expected 3 listings (1 global + 2 targeted), got %d", len(result))
	}
}

func TestFetchTargetedListings_ContextCanceled(t *testing.T) {
	sr1 := storage.Search{ID: 1, Manufacturer: 27, Model: 10332}
	sr2 := storage.Search{ID: 2, Manufacturer: 19, Model: 10222}
	key1 := paramKeyStr(model.SourceParamsFromSearch(&sr1))
	key2 := paramKeyStr(model.SourceParamsFromSearch(&sr2))

	f := &paramAwareFetcher{
		results: map[string][]model.RawListing{
			key1: {{Token: "t1"}},
			key2: {{Token: "t2"}},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	global := []model.RawListing{{Token: "g1"}}
	result, _ := s.fetchTargetedListings(ctx, []storage.Search{sr1, sr2}, global, f)

	if len(result) != len(global) {
		t.Errorf("expected %d listings (global only), got %d", len(global), len(result))
	}
	for _, l := range result {
		if l.Token == "t1" || l.Token == "t2" {
			t.Errorf("targeted token %q should not appear after context cancellation", l.Token)
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("expected 0 fetch calls after context cancellation, got %d", len(f.calls))
	}
}
