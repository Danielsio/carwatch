package winwin

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

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

	proxyMu      sync.Mutex
	cachedProxy  string
	cachedClient *Client
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
	var usedProxy string
	if f.proxyPool != nil {
		proxyURL := f.proxyPool.Next()
		usedProxy = proxyURL
		f.proxyMu.Lock()
		if proxyURL == f.cachedProxy && f.cachedClient != nil {
			cli = f.cachedClient
			f.proxyMu.Unlock()
		} else {
			c, err := NewClient(f.userAgents, proxyURL)
			if err != nil {
				f.proxyMu.Unlock()
				return nil, fmt.Errorf("winwin proxy client: %w", err)
			}
			if f.cachedClient != nil {
				f.cachedClient.CloseIdleConnections()
			}
			f.cachedProxy = proxyURL
			f.cachedClient = c
			cli = c
			f.proxyMu.Unlock()
		}
	}
	reqURL := buildURL(f.baseURL, params)
	f.logger.Info("fetching winwin listings",
		"url", reqURL,
		"manufacturer", params.Manufacturer,
		"model", params.Model,
	)
	result, err := cli.Get(ctx, reqURL)
	if err != nil {
		f.evictOnFailure(usedProxy)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	f.logger.Debug("winwin response received",
		"status", result.StatusCode,
		"body_bytes", len(result.Body),
	)
	switch result.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		f.evictOnFailure(usedProxy)
		return nil, fetcher.ErrRateLimited
	default:
		if looksLikeChallenge(string(result.Body)) {
			f.evictOnFailure(usedProxy)
			return nil, fetcher.ErrChallenge
		}
		body := string(result.Body)
		if len(body) > 512 {
			body = body[:512] + "…"
		}
		f.evictOnFailure(usedProxy)
		return nil, fmt.Errorf("unexpected status %d: %s", result.StatusCode, body)
	}
	listings, err := ParseListingsPage(bytes.NewReader(result.Body))
	if err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}
	f.logger.Info("fetched winwin listings", "count", len(listings))
	for i, l := range listings {
		f.logger.Debug("winwin listing parsed",
			"idx", i,
			"token", l.Token,
			"manufacturer", l.Manufacturer,
			"model", l.Model,
			"year", l.Year,
			"price", l.Price,
		)
	}
	return listings, nil
}

func (f *WinWinFetcher) evictOnFailure(proxyURL string) {
	if f.proxyPool == nil {
		return
	}
	if proxyURL != "" {
		f.proxyPool.MarkUnhealthy(proxyURL)
	}
	f.proxyMu.Lock()
	if f.cachedProxy == proxyURL {
		f.cachedClient = nil
		f.cachedProxy = ""
	}
	f.proxyMu.Unlock()
}
