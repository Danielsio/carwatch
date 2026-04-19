package winwin

import (
	"context"
	"log/slog"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

// WinWinFetcher implements the fetcher.Fetcher interface for winwin.co.il.
type WinWinFetcher struct {
	client    *Client
	baseURL   string
	logger    *slog.Logger
	proxyPool *fetcher.ProxyPool
}

// NewFetcher creates a WinWin fetcher with optional proxy support.
func NewFetcher(userAgents []string, proxy string, logger *slog.Logger) (*WinWinFetcher, error) {
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	return &WinWinFetcher{
		client:  client,
		baseURL: defaultBaseURL,
		logger:  logger,
	}, nil
}

// NewFetcherWithProxyPool creates a WinWin fetcher with rotating proxy support.
func NewFetcherWithProxyPool(userAgents []string, pool *fetcher.ProxyPool, logger *slog.Logger) (*WinWinFetcher, error) {
	proxy := ""
	if pool != nil {
		proxy = pool.Next()
	}
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	return &WinWinFetcher{
		client:    client,
		baseURL:   defaultBaseURL,
		logger:    logger,
		proxyPool: pool,
	}, nil
}

// Fetch retrieves car listings from WinWin.
// TODO: Implement actual scraping logic once the WinWin API/page structure
// is reverse-engineered. For now this returns empty results so the fetcher
// can be registered and tested end-to-end without hitting a real server.
func (f *WinWinFetcher) Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error) {
	reqURL := buildURL(f.baseURL, params)
	f.logger.Info("winwin fetch stub", "url", reqURL)

	// TODO: Make HTTP request, parse response, return listings.
	// The full implementation requires reverse-engineering the WinWin
	// HTML structure or discovering a JSON API endpoint.
	return nil, nil
}
