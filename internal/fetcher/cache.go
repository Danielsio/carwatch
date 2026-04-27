package fetcher

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

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
	lastUsed  time.Time
}

func NewCachingFetcher(inner Fetcher, ttl time.Duration) *CachingFetcher {
	return &CachingFetcher{
		inner: inner,
		ttl:   ttl,
		cache: make(map[string]cacheEntry),
	}
}

func (c *CachingFetcher) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	key := cacheKey(params)

	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < c.ttl {
		c.touch(key)
		return entry.listings, nil
	}

	listings, err := c.inner.Fetch(ctx, params)
	if err != nil {
		if errors.Is(err, ErrChallenge) || errors.Is(err, ErrRateLimited) {
			return nil, err
		}
		if errors.Is(err, ErrPartialResults) {
			return listings, err
		}
		if ok {
			c.touch(key)
			return entry.listings, nil
		}
		return nil, err
	}

	c.mu.Lock()
	now := time.Now()
	c.cache[key] = cacheEntry{listings: listings, fetchedAt: now, lastUsed: now}
	if len(c.cache) > maxCacheEntries {
		c.evictExpired()
	}
	if len(c.cache) > maxCacheEntries {
		c.evictOldest()
	}
	c.mu.Unlock()

	return listings, nil
}

const maxCacheEntries = 100

func (c *CachingFetcher) evictExpired() {
	for key, entry := range c.cache {
		if time.Since(entry.fetchedAt) > 2*c.ttl {
			delete(c.cache, key)
		}
	}
}

func (c *CachingFetcher) evictOldest() {
	type kv struct {
		key      string
		lastUsed time.Time
	}
	items := make([]kv, 0, len(c.cache))
	for k, v := range c.cache {
		items = append(items, kv{k, v.lastUsed})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].lastUsed.Before(items[j].lastUsed)
	})
	for _, item := range items {
		if len(c.cache) <= maxCacheEntries {
			break
		}
		delete(c.cache, item.key)
	}
}

func (c *CachingFetcher) touch(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, exists := c.cache[key]; exists {
		cur.lastUsed = time.Now()
		c.cache[key] = cur
	}
}

func cacheKey(p model.SourceParams) string {
	return fmt.Sprintf("%d:%d:%d-%d:%d-%d:%d",
		p.Manufacturer, p.Model, p.YearMin, p.YearMax, p.PriceMin, p.PriceMax, p.Page)
}
