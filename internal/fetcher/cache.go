package fetcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

type CachingFetcher struct {
	inner Fetcher
	ttl   time.Duration
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	listings  []model.RawListing
	fetchedAt time.Time
}

func NewCachingFetcher(inner Fetcher, ttl time.Duration) *CachingFetcher {
	return &CachingFetcher{
		inner: inner,
		ttl:   ttl,
		cache: make(map[string]cacheEntry),
	}
}

func (c *CachingFetcher) Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error) {
	key := cacheKey(params)

	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < c.ttl {
		return entry.listings, nil
	}

	listings, err := c.inner.Fetch(ctx, params)
	if err != nil {
		if errors.Is(err, ErrChallenge) || errors.Is(err, ErrRateLimited) {
			return nil, err
		}
		if ok {
			return entry.listings, nil
		}
		return nil, err
	}

	c.mu.Lock()
	c.cache[key] = cacheEntry{listings: listings, fetchedAt: time.Now()}
	c.mu.Unlock()

	return listings, nil
}

func cacheKey(p config.SourceParams) string {
	return fmt.Sprintf("%d:%d:%d-%d:%d-%d:%d",
		p.Manufacturer, p.Model, p.YearMin, p.YearMax, p.PriceMin, p.PriceMax, p.Page)
}
