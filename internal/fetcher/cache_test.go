package fetcher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

type countingFetcher struct {
	calls    atomic.Int32
	listings []model.RawListing
	err      error
}

func (f *countingFetcher) Fetch(_ context.Context, _ model.SourceParams) ([]model.RawListing, error) {
	f.calls.Add(1)
	return f.listings, f.err
}

func TestCachingFetcher_CachesWithinTTL(t *testing.T) {
	inner := &countingFetcher{
		listings: []model.RawListing{{Token: "a"}},
	}
	cf := NewCachingFetcher(inner, time.Hour)

	params := model.SourceParams{Manufacturer: 1}
	ctx := context.Background()

	r1, err := cf.Fetch(ctx, params)
	if err != nil || len(r1) != 1 {
		t.Fatalf("first fetch: err=%v, len=%d", err, len(r1))
	}

	r2, err := cf.Fetch(ctx, params)
	if err != nil || len(r2) != 1 {
		t.Fatalf("second fetch: err=%v, len=%d", err, len(r2))
	}

	if inner.calls.Load() != 1 {
		t.Errorf("inner fetcher called %d times, want 1 (cached)", inner.calls.Load())
	}
}

func TestCachingFetcher_FallsBackOnError(t *testing.T) {
	inner := &countingFetcher{
		listings: []model.RawListing{{Token: "a"}},
	}
	cf := NewCachingFetcher(inner, time.Nanosecond)

	ctx := context.Background()
	params := model.SourceParams{Manufacturer: 1}

	_, _ = cf.Fetch(ctx, params)
	time.Sleep(time.Millisecond)

	inner.err = errors.New("network error")
	inner.listings = nil

	result, err := cf.Fetch(ctx, params)
	if err != nil {
		t.Fatalf("expected stale cache fallback, got error: %v", err)
	}
	if len(result) != 1 || result[0].Token != "a" {
		t.Errorf("expected stale cached result, got %v", result)
	}
}

func TestCachingFetcher_EvictsOldest(t *testing.T) {
	inner := &countingFetcher{
		listings: []model.RawListing{{Token: "x"}},
	}
	cf := NewCachingFetcher(inner, time.Hour)
	ctx := context.Background()

	// Fill cache to capacity with distinct lastUsed timestamps so
	// eviction order is deterministic regardless of clock resolution.
	base := time.Now().Add(-time.Hour)
	for i := 0; i < maxCacheEntries; i++ {
		params := model.SourceParams{Manufacturer: i, Model: 1}
		if _, err := cf.Fetch(ctx, params); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		key := cacheKey(params)
		entry := cf.cache[key]
		entry.lastUsed = base.Add(time.Duration(i) * time.Second)
		cf.cache[key] = entry
	}

	// Touch key 0 to give it the newest lastUsed.
	touchedParams := model.SourceParams{Manufacturer: 0, Model: 1}
	if _, err := cf.Fetch(ctx, touchedParams); err != nil {
		t.Fatalf("touch fetch: %v", err)
	}

	// Add entries past capacity to force eviction.
	for i := maxCacheEntries; i < maxCacheEntries+20; i++ {
		params := model.SourceParams{Manufacturer: i, Model: 1}
		if _, err := cf.Fetch(ctx, params); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}

	cf.mu.RLock()
	size := len(cf.cache)
	touchedKey := cacheKey(touchedParams)
	_, touchedExists := cf.cache[touchedKey]
	untouchedKey := cacheKey(model.SourceParams{Manufacturer: 1, Model: 1})
	_, untouchedExists := cf.cache[untouchedKey]
	cf.mu.RUnlock()

	if size > maxCacheEntries {
		t.Errorf("cache size = %d, want <= %d", size, maxCacheEntries)
	}
	if !touchedExists {
		t.Error("touched key should survive eviction (LRU)")
	}
	if untouchedExists {
		t.Error("untouched early key should have been evicted (LRU)")
	}
}
