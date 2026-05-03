package winwin

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

// WinWinFetcher implements the fetcher.Fetcher interface for winwin.co.il.
type WinWinFetcher struct {
	userAgents []string
	client     *Client
	baseURL    string
	logger     *slog.Logger
	proxyPool  *fetcher.ProxyPool
}

// NewFetcher creates a WinWin fetcher with optional proxy support.
func NewFetcher(userAgents []string, proxy string, logger *slog.Logger) (*WinWinFetcher, error) {
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	return &WinWinFetcher{
		userAgents: userAgents,
		client:     client,
		baseURL:    defaultBaseURL,
		logger:     logger,
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
		userAgents: userAgents,
		client:     client,
		baseURL:    defaultBaseURL,
		logger:     logger,
		proxyPool:  pool,
	}, nil
}

// Fetch retrieves car listings from WinWin.
func (f *WinWinFetcher) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	cli := f.client
	if f.proxyPool != nil {
		c, err := NewClient(f.userAgents, f.proxyPool.Next())
		if err != nil {
			return nil, fmt.Errorf("winwin proxy client: %w", err)
		}
		cli = c
	}
	reqURL := buildURL(f.baseURL, params)
	f.logger.Info("fetching winwin listings", "url", reqURL)
	result, err := cli.Get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	switch result.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		return nil, fetcher.ErrRateLimited
	default:
		if looksLikeChallenge(string(result.Body)) {
			return nil, fetcher.ErrChallenge
		}
		return nil, fmt.Errorf("unexpected status: %d", result.StatusCode)
	}
	listings, err := ParseListingsPage(bytes.NewReader(result.Body))
	if err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}
	f.logger.Info("fetched winwin listings", "count", len(listings))
	return listings, nil
}
