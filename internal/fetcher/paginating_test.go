package fetcher

import (
	"context"
	"errors"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

type pageMockFetcher struct {
	pages map[int][]model.RawListing
	err   error
	calls []int
}

func (m *pageMockFetcher) Fetch(_ context.Context, params config.SourceParams) ([]model.RawListing, error) {
	m.calls = append(m.calls, params.Page)
	if m.err != nil {
		return nil, m.err
	}
	return m.pages[params.Page], nil
}

func TestPaginatingFetcher_SinglePage(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}, {Token: "b"}},
		},
	}
	pf := NewPaginatingFetcher(inner, 5)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 listings, got %d", len(listings))
	}
}

func TestPaginatingFetcher_MultiplePages(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}, {Token: "b"}},
			2: {{Token: "c"}, {Token: "d"}},
			3: {{Token: "e"}},
		},
	}
	pf := NewPaginatingFetcher(inner, 5)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 5 {
		t.Errorf("expected 5 listings across 3 pages, got %d", len(listings))
	}
}

func TestPaginatingFetcher_StopsAtEmptyPage(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}},
			2: {},
		},
	}
	pf := NewPaginatingFetcher(inner, 5)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing, got %d", len(listings))
	}
	if len(inner.calls) != 2 {
		t.Errorf("expected 2 calls (page 1 + empty page 2), got %d", len(inner.calls))
	}
}

func TestPaginatingFetcher_RespectsMaxPages(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}},
			2: {{Token: "b"}},
			3: {{Token: "c"}},
			4: {{Token: "d"}},
			5: {{Token: "e"}},
		},
	}
	pf := NewPaginatingFetcher(inner, 3)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 3 {
		t.Errorf("expected 3 listings (max 3 pages), got %d", len(listings))
	}
	if len(inner.calls) != 3 {
		t.Errorf("expected 3 fetch calls, got %d", len(inner.calls))
	}
}

func TestPaginatingFetcher_DeduplicatesAcrossPages(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}, {Token: "b"}},
			2: {{Token: "b"}, {Token: "c"}},
			3: {{Token: "c"}, {Token: "d"}},
		},
	}
	pf := NewPaginatingFetcher(inner, 5)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 4 {
		t.Errorf("expected 4 unique listings, got %d", len(listings))
	}
}

func TestPaginatingFetcher_StopsWhenAllDuplicates(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}, {Token: "b"}},
			2: {{Token: "a"}, {Token: "b"}},
		},
	}
	pf := NewPaginatingFetcher(inner, 5)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 unique listings, got %d", len(listings))
	}
	if len(inner.calls) != 2 {
		t.Errorf("should stop after all-duplicate page, got %d calls", len(inner.calls))
	}
}

func TestPaginatingFetcher_FirstPageError_Propagates(t *testing.T) {
	inner := &pageMockFetcher{err: errors.New("network error")}
	pf := NewPaginatingFetcher(inner, 5)

	_, err := pf.Fetch(context.Background(), config.SourceParams{})
	if err == nil {
		t.Fatal("expected error for first page failure")
	}
	if errors.Is(err, ErrPartialResults) {
		t.Error("first page error should not be ErrPartialResults")
	}
}

type laterErrorFetcher struct {
	pages   map[int][]model.RawListing
	errPage int
	calls   []int
}

func (m *laterErrorFetcher) Fetch(_ context.Context, params config.SourceParams) ([]model.RawListing, error) {
	m.calls = append(m.calls, params.Page)
	if params.Page == m.errPage {
		return nil, errors.New("page error")
	}
	return m.pages[params.Page], nil
}

func TestPaginatingFetcher_LaterPageError_ReturnsPartialWithError(t *testing.T) {
	inner := &laterErrorFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}},
		},
		errPage: 2,
	}
	pf := NewPaginatingFetcher(inner, 5)

	listings, err := pf.Fetch(context.Background(), config.SourceParams{})
	if !errors.Is(err, ErrPartialResults) {
		t.Fatalf("expected ErrPartialResults, got: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing from page 1, got %d", len(listings))
	}
	if len(inner.calls) < 2 || inner.calls[1] != 2 {
		t.Fatalf("expected fetch to reach page 2, calls=%v", inner.calls)
	}
}

func TestPaginatingFetcher_PassesParams(t *testing.T) {
	inner := &pageMockFetcher{
		pages: map[int][]model.RawListing{
			1: {{Token: "a"}},
		},
	}
	pf := NewPaginatingFetcher(inner, 5)

	_, _ = pf.Fetch(context.Background(), config.SourceParams{Manufacturer: 27, Model: 10332})
	if len(inner.calls) < 1 {
		t.Fatal("expected at least 1 call")
	}
	if inner.calls[0] != 1 {
		t.Errorf("first call page = %d, want 1", inner.calls[0])
	}
}

func TestPaginatingFetcher_DefaultMaxPages(t *testing.T) {
	pf := NewPaginatingFetcher(nil, 0)
	if pf.maxPages != DefaultMaxPages {
		t.Errorf("maxPages = %d, want %d", pf.maxPages, DefaultMaxPages)
	}

	pf = NewPaginatingFetcher(nil, -1)
	if pf.maxPages != DefaultMaxPages {
		t.Errorf("negative maxPages should default to %d, got %d", DefaultMaxPages, pf.maxPages)
	}
}
