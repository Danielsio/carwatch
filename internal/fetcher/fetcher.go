package fetcher

import (
	"context"
	"errors"
	"sync"

	"github.com/dsionov/carwatch/internal/model"
)

var (
	ErrChallenge   = errors.New("anti-bot challenge detected")
	ErrRateLimited = errors.New("rate limited")
)

type Fetcher interface {
	Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error)
}

type Factory struct {
	mu       sync.RWMutex
	fetchers map[string]Fetcher
}

func NewFactory() *Factory {
	return &Factory{fetchers: make(map[string]Fetcher)}
}

func (f *Factory) Register(source string, fetcher Fetcher) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchers[source] = fetcher
}

func (f *Factory) Get(source string) (Fetcher, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	fetcher, ok := f.fetchers[source]
	return fetcher, ok
}
