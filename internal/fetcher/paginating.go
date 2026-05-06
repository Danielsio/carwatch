package fetcher

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

const (
	DefaultMaxPages  = 5
	defaultPageDelay = 1500 * time.Millisecond
	pageDelayJitter  = 1000 * time.Millisecond
)

var ErrPartialResults = fmt.Errorf("partial paginated results")

type PaginatingFetcher struct {
	inner     Fetcher
	maxPages  int
	pageDelay time.Duration
}

func NewPaginatingFetcher(inner Fetcher, maxPages int) *PaginatingFetcher {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	return &PaginatingFetcher{inner: inner, maxPages: maxPages, pageDelay: defaultPageDelay}
}

func (f *PaginatingFetcher) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	seen := make(map[string]bool)
	var all []model.RawListing

	for page := 1; page <= f.maxPages; page++ {
		if page > 1 && f.pageDelay > 0 {
			jitter := time.Duration(rand.Int64N(int64(pageDelayJitter)))
			delay := f.pageDelay + jitter
			select {
			case <-ctx.Done():
				if len(all) > 0 {
					return all, fmt.Errorf("%w: canceled after page %d: %v", ErrPartialResults, page-1, ctx.Err())
				}
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		p := params
		p.Page = page

		listings, err := f.inner.Fetch(ctx, p)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			return all, fmt.Errorf("%w: page %d: %v", ErrPartialResults, page, err)
		}

		if len(listings) == 0 {
			break
		}

		added := 0
		for _, l := range listings {
			if !seen[l.Token] {
				seen[l.Token] = true
				all = append(all, l)
				added++
			}
		}

		if added == 0 {
			break
		}
	}

	return all, nil
}
