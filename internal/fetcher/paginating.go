package fetcher

import (
	"context"
	"fmt"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

const DefaultMaxPages = 5

var ErrPartialResults = fmt.Errorf("partial paginated results")

type PaginatingFetcher struct {
	inner    Fetcher
	maxPages int
}

func NewPaginatingFetcher(inner Fetcher, maxPages int) *PaginatingFetcher {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	return &PaginatingFetcher{inner: inner, maxPages: maxPages}
}

func (f *PaginatingFetcher) Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error) {
	seen := make(map[string]bool)
	var all []model.RawListing

	for page := 1; page <= f.maxPages; page++ {
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
