package scheduler

import (
	"context"
	"sync"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// paramAwareFetcher returns different listings based on SourceParams.
type paramAwareFetcher struct {
	mu       sync.Mutex
	calls    []model.SourceParams
	results  map[paramKey][]model.RawListing
	errors   map[paramKey]error
	fallback []model.RawListing
}

type paramKey struct{ Manufacturer, Model int }

func (f *paramAwareFetcher) Fetch(_ context.Context, p model.SourceParams) ([]model.RawListing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, p)
	key := paramKey{p.Manufacturer, p.Model}
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
	s, _ := NewWithOptions(cfg, f, newMockDedup(), &mockNotifier{}, testLogger(), Options{})
	return s
}

func TestFetchTargetedListings_NoPairsNoFetch(t *testing.T) {
	f := &paramAwareFetcher{}
	s := newTestSchedulerWithFetcher(f)

	searches := []storage.Search{
		{ID: 1, Manufacturer: 0, Model: 0},
	}
	global := []model.RawListing{{Token: "g1"}}
	result := s.fetchTargetedListings(context.Background(), searches, global, f)

	if len(result) != 1 {
		t.Errorf("expected 1 listing (unchanged), got %d", len(result))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("expected 0 fetch calls for wildcard-only searches, got %d", len(f.calls))
	}
}

func TestFetchTargetedListings_FetchesUncoveredPair(t *testing.T) {
	f := &paramAwareFetcher{
		results: map[paramKey][]model.RawListing{
			{0, 0}: {{Token: "g1", ManufacturerID: 99, ModelID: 999}},
			{27, 10332}: {
				{Token: "t1", ManufacturerID: 27, ModelID: 10332},
				{Token: "t2", ManufacturerID: 27, ModelID: 10332},
			},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	searches := []storage.Search{
		{ID: 1, Manufacturer: 27, Model: 10332},
	}
	global := []model.RawListing{{Token: "g1", ManufacturerID: 99, ModelID: 999}}
	result := s.fetchTargetedListings(context.Background(), searches, global, f)

	if len(result) != 3 {
		t.Errorf("expected 3 listings (1 global + 2 targeted), got %d", len(result))
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 targeted fetch call, got %d", len(f.calls))
	}
	if f.calls[0].Manufacturer != 27 || f.calls[0].Model != 10332 {
		t.Errorf("targeted fetch used wrong params: %+v", f.calls[0])
	}
}

func TestFetchTargetedListings_SkipsWellCoveredPair(t *testing.T) {
	f := &paramAwareFetcher{}
	s := newTestSchedulerWithFetcher(f)

	searches := []storage.Search{
		{ID: 1, Manufacturer: 27, Model: 10332},
	}
	// Global feed already has 5 listings for this pair.
	global := make([]model.RawListing, 5)
	for i := range global {
		global[i] = model.RawListing{
			Token:          "g" + string(rune('0'+i)),
			ManufacturerID: 27,
			ModelID:        10332,
		}
	}

	result := s.fetchTargetedListings(context.Background(), searches, global, f)

	if len(result) != 5 {
		t.Errorf("expected 5 listings (no targeted fetch), got %d", len(result))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Errorf("expected 0 fetch calls (pair well-covered), got %d", len(f.calls))
	}
}

func TestFetchTargetedListings_DeduplicatesTokens(t *testing.T) {
	f := &paramAwareFetcher{
		results: map[paramKey][]model.RawListing{
			{17, 10182}: {
				{Token: "shared", ManufacturerID: 17, ModelID: 10182},
				{Token: "new1", ManufacturerID: 17, ModelID: 10182},
			},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	searches := []storage.Search{
		{ID: 1, Manufacturer: 17, Model: 10182},
	}
	global := []model.RawListing{{Token: "shared", ManufacturerID: 17, ModelID: 10182}}

	result := s.fetchTargetedListings(context.Background(), searches, global, f)

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
	f := &paramAwareFetcher{
		results: map[paramKey][]model.RawListing{
			{19, 10222}: {{Token: "ok1", ManufacturerID: 19, ModelID: 10222}},
		},
		errors: map[paramKey]error{
			{27, 10332}: fetcher.ErrChallenge,
		},
	}
	s := newTestSchedulerWithFetcher(f)

	searches := []storage.Search{
		{ID: 1, Manufacturer: 27, Model: 10332},
		{ID: 2, Manufacturer: 19, Model: 10222},
	}
	global := []model.RawListing{{Token: "g1"}}

	result := s.fetchTargetedListings(context.Background(), searches, global, f)

	// Should have global + successful targeted fetch, despite first pair failing.
	hasOk1 := false
	for _, l := range result {
		if l.Token == "ok1" {
			hasOk1 = true
		}
	}
	if !hasOk1 {
		t.Error("expected targeted listing 'ok1' from successful pair, not found")
	}
}

func TestFetchTargetedListings_SharedPairFetchedOnce(t *testing.T) {
	f := &paramAwareFetcher{
		results: map[paramKey][]model.RawListing{
			{27, 10332}: {{Token: "t1", ManufacturerID: 27, ModelID: 10332}},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	// Two searches for the same manufacturer/model pair.
	searches := []storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332},
		{ID: 2, ChatID: 200, Manufacturer: 27, Model: 10332},
	}
	global := []model.RawListing{{Token: "g1"}}

	result := s.fetchTargetedListings(context.Background(), searches, global, f)

	f.mu.Lock()
	defer f.mu.Unlock()

	targetedCalls := 0
	for _, c := range f.calls {
		if c.Manufacturer == 27 && c.Model == 10332 {
			targetedCalls++
		}
	}
	if targetedCalls != 1 {
		t.Errorf("expected 1 targeted fetch for shared pair, got %d", targetedCalls)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 listings (1 global + 1 targeted), got %d", len(result))
	}
}

func TestFetchTargetedListings_ContextCanceled(t *testing.T) {
	f := &paramAwareFetcher{
		results: map[paramKey][]model.RawListing{
			{27, 10332}: {{Token: "t1"}},
			{19, 10222}: {{Token: "t2"}},
		},
	}
	s := newTestSchedulerWithFetcher(f)

	searches := []storage.Search{
		{ID: 1, Manufacturer: 27, Model: 10332},
		{ID: 2, Manufacturer: 19, Model: 10222},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	global := []model.RawListing{{Token: "g1"}}
	result := s.fetchTargetedListings(ctx, searches, global, f)

	// With context already cancelled, the method should return early.
	// Global listings must always be preserved.
	hasGlobal := false
	for _, l := range result {
		if l.Token == "g1" {
			hasGlobal = true
		}
	}
	if !hasGlobal {
		t.Error("global listing 'g1' missing from result after context cancellation")
	}
}
