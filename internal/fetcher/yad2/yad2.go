package yad2

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

const (
	defaultBaseURL  = "https://www.yad2.co.il/vehicles/cars"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

type Yad2Fetcher struct {
	client     HTTPDoer
	baseURL    string
	logger     *slog.Logger
	userAgents []string
	proxyPool  *fetcher.ProxyPool
	clientPool *ClientPool
}

func NewFetcher(userAgents []string, proxy string, logger *slog.Logger) (*Yad2Fetcher, error) {
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	return &Yad2Fetcher{client: client, baseURL: defaultBaseURL, logger: logger, userAgents: userAgents}, nil
}

func NewFetcherWithProxyPool(userAgents []string, pool *fetcher.ProxyPool, logger *slog.Logger) (*Yad2Fetcher, error) {
	proxy := ""
	if pool != nil {
		proxy = pool.Next()
	}
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	var cp *ClientPool
	if pool != nil {
		cp = NewClientPool(userAgents, logger)
	}
	return &Yad2Fetcher{
		client:     client,
		baseURL:    defaultBaseURL,
		logger:     logger,
		userAgents: userAgents,
		proxyPool:  pool,
		clientPool: cp,
	}, nil
}

func (f *Yad2Fetcher) Close() {
	if f.clientPool != nil {
		f.clientPool.Close()
	}
	if f.client != nil {
		f.client.Close()
	}
}

// FetchItem fetches an individual listing page and returns enrichment details.
func (f *Yad2Fetcher) FetchItem(ctx context.Context, token string) (ItemDetails, error) {
	base, err := url.Parse(f.baseURL)
	if err != nil {
		return ItemDetails{}, fmt.Errorf("parse base URL: %w", err)
	}
	base.Path = "/vehicles/item/" + url.PathEscape(token)
	base.RawQuery = ""
	itemURL := base.String()

	client := f.client
	var usedProxy string
	if f.proxyPool != nil {
		usedProxy = f.proxyPool.Next()
		if f.clientPool != nil {
			c, err := f.clientPool.Get(usedProxy)
			if err != nil {
				f.logger.Warn("failed to get pooled client for item fetch", "proxy", redactProxy(usedProxy), "error", err)
			} else {
				client = c
			}
		}
	}

	result, err := client.Get(ctx, itemURL)
	if err != nil {
		if f.clientPool != nil && usedProxy != "" {
			f.clientPool.Evict(usedProxy)
		}
		return ItemDetails{}, fmt.Errorf("fetch item %s: %w", token, err)
	}

	if result.StatusCode != http.StatusOK {
		return ItemDetails{}, fmt.Errorf("fetch item %s: status %d", token, result.StatusCode)
	}

	details, err := ParseItemPage(bytes.NewReader(result.Body))
	if err != nil {
		return ItemDetails{}, fmt.Errorf("parse item %s: %w", token, err)
	}

	return details, nil
}

func (f *Yad2Fetcher) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	client := f.client
	var usedProxy string
	if f.proxyPool != nil {
		usedProxy = f.proxyPool.Next()
		if f.clientPool != nil {
			c, err := f.clientPool.Get(usedProxy)
			if err != nil {
				f.logger.Warn("failed to get pooled client, using fallback", "proxy", redactProxy(usedProxy), "error", err)
			} else {
				client = c
			}
		}
	}

	reqURL := buildURL(f.baseURL, params)
	f.logger.Info("fetching listings", "url", reqURL)

	result, err := client.Get(ctx, reqURL)
	if err != nil {
		if f.clientPool != nil && usedProxy != "" {
			f.clientPool.Evict(usedProxy)
		}
		return nil, fmt.Errorf("execute request: %w", err)
	}

	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", result.StatusCode)
	}

	listings, err := ParseListingsPageWithLogger(bytes.NewReader(result.Body), f.logger)
	if err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}

	f.logger.Info("fetched listings", "count", len(listings))
	return listings, nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	reader := io.LimitReader(resp.Body, maxResponseSize)
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer func() { _ = gr.Close() }()
		reader = gr
	}
	return io.ReadAll(reader)
}

func buildURL(base string, params model.SourceParams) string {
	u, _ := url.Parse(base)
	v := url.Values{}

	if params.Manufacturer > 0 {
		v.Set("manufacturer", strconv.Itoa(params.Manufacturer))
	}
	if params.Model > 0 {
		v.Set("model", strconv.Itoa(params.Model))
	}
	if params.YearMin > 0 || params.YearMax > 0 {
		yearMin := params.YearMin
		if yearMin == 0 {
			yearMin = 2000
		}
		yearMax := params.YearMax
		if yearMax == 0 {
			yearMax = 2030
		}
		v.Set("year", strconv.Itoa(yearMin)+"-"+strconv.Itoa(yearMax))
	}
	if params.PriceMin > 0 || params.PriceMax > 0 {
		priceMin := params.PriceMin
		priceMax := params.PriceMax
		if priceMax == 0 {
			priceMax = 9999999
		}
		v.Set("price", strconv.Itoa(priceMin)+"-"+strconv.Itoa(priceMax))
	}
	v.Set("Order", "1")
	if params.Page > 0 {
		v.Set("page", strconv.Itoa(params.Page))
	}

	u.RawQuery = v.Encode()
	return u.String()
}
