package fetcher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

type countingFetcher struct {
	calls    atomic.Int32
	listings []model.RawListing
	err      error
}

func (f *countingFetcher) Fetch(_ context.Context, _ config.SourceParams) ([]model.RawListing, error) {
	f.calls.Add(1)
	return f.listings, f.err
}

func TestCachingFetcher_CachesWithinTTL(t *testing.T) {
	inner := &countingFetcher{
		listings: []model.RawListing{{Token: "a"}},
	}
	cf := NewCachingFetcher(inner, time.Hour)

	params := config.SourceParams{Manufacturer: 1}
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
	params := config.SourceParams{Manufacturer: 1}

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
