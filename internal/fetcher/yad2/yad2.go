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
	"time"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

const (
	defaultBaseURL  = "https://www.yad2.co.il/vehicles/cars"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

type Yad2Fetcher struct {
	client     HTTPDoer // listing pages only
	itemClient HTTPDoer // item pages only (isolated cookie jar)
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
	ic, err := NewClient(userAgents, proxy)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &Yad2Fetcher{client: client, itemClient: ic, baseURL: defaultBaseURL, logger: logger, userAgents: userAgents}, nil
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
	ic, err := NewClient(userAgents, proxy)
	if err != nil {
		client.Close()
		return nil, err
	}
	var cp *ClientPool
	if pool != nil {
		cp = NewClientPool(userAgents, logger)
	}
	return &Yad2Fetcher{
		client:     client,
		itemClient: ic,
		baseURL:    defaultBaseURL,
		logger:     logger,
		userAgents: userAgents,
		proxyPool:  pool,
		clientPool: cp,
	}, nil
}

// FetchRawPage fetches a URL and returns the raw response body.
// Used by the catalog loader to fetch the Yad2 cars page.
func (f *Yad2Fetcher) FetchRawPage(ctx context.Context, rawURL string) ([]byte, error) {
	result, err := f.client.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if result.StatusCode != http.StatusOK {
		if looksLikeBotProtection(result.Body) {
			return nil, fmt.Errorf("yad2 raw page: %w", fetcher.ErrChallenge)
		}
		return nil, fmt.Errorf("unexpected status %d", result.StatusCode)
	}
	return result.Body, nil
}

func (f *Yad2Fetcher) Close() {
	if f.clientPool != nil {
		f.clientPool.Close()
	}
	if f.itemClient != nil {
		f.itemClient.Close()
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

	client := f.itemClient
	var usedProxy string
	if f.proxyPool != nil {
		usedProxy = f.proxyPool.Next()
		if f.clientPool != nil {
			c, err := f.clientPool.Get(usedProxy)
			if err != nil {
				f.logger.Warn("failed to get pooled client for item fetch, using fallback", "proxy", redactProxy(usedProxy), "error", err)
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
		if looksLikeBotProtection(result.Body) {
			if f.proxyPool != nil && usedProxy != "" {
				f.proxyPool.MarkUnhealthy(usedProxy)
			}
			return ItemDetails{}, fmt.Errorf("fetch item %s: %w", token, fetcher.ErrChallenge)
		}
		if (result.StatusCode == http.StatusBadRequest || result.StatusCode == http.StatusForbidden) &&
			looksLikeGenericError(result.Body) {
			if f.proxyPool != nil && usedProxy != "" {
				f.proxyPool.MarkUnhealthy(usedProxy)
			}
			return ItemDetails{}, fmt.Errorf("fetch item %s: %w", token, fetcher.ErrChallenge)
		}
		if result.StatusCode == http.StatusNotFound || result.StatusCode == http.StatusGone {
			// The listing was removed from Yad2; retrying this token is futile.
			return ItemDetails{}, fmt.Errorf("fetch item %s: status %d: %w", token, result.StatusCode, fetcher.ErrItemGone)
		}
		snippet := string(result.Body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		f.logger.Debug("item fetch non-200 response",
			"token", token,
			"status", result.StatusCode,
			"body_preview", snippet,
		)
		return ItemDetails{}, fmt.Errorf("fetch item %s: status %d", token, result.StatusCode)
	}

	details, err := ParseItemPage(bytes.NewReader(result.Body))
	if err != nil {
		return ItemDetails{}, fmt.Errorf("parse item %s: %w", token, err)
	}

	f.logger.Debug("yad2 item page parsed",
		"token", token,
		"km", details.Km,
		"city", details.City,
		"area", details.Area,
		"has_cover_image", details.ImageURL != "",
	)

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
	fetchStart := time.Now()
	f.logger.Debug("fetching listings",
		"url", reqURL,
		"manufacturer", params.Manufacturer,
		"model", params.Model,
		"page", params.Page,
	)

	result, err := client.Get(ctx, reqURL)
	if err != nil {
		if f.clientPool != nil && usedProxy != "" {
			f.clientPool.Evict(usedProxy)
		}
		f.logger.Error("fetch request failed",
			"url", reqURL,
			"manufacturer", params.Manufacturer, "model", params.Model,
			"duration_ms", time.Since(fetchStart).Milliseconds(), "error", err)
		return nil, fmt.Errorf("execute request: %w", err)
	}

	if result.StatusCode != http.StatusOK {
		if looksLikeBotProtection(result.Body) {
			if f.proxyPool != nil && usedProxy != "" {
				f.proxyPool.MarkUnhealthy(usedProxy)
			}
			return nil, fmt.Errorf("yad2: %w", fetcher.ErrChallenge)
		}
		if result.StatusCode == http.StatusBadRequest || result.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("yad2 status %d: %w", result.StatusCode, fetcher.ErrRateLimited)
		}
		body := string(result.Body)
		if len(body) > 512 {
			body = body[:512] + "…"
		}
		return nil, fmt.Errorf("unexpected status %d: %s", result.StatusCode, body)
	}

	listings, err := ParseListingsPageWithLogger(bytes.NewReader(result.Body), f.logger)
	if err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}

	f.logger.Debug("fetched listings",
		"count", len(listings),
		"manufacturer", params.Manufacturer,
		"model", params.Model,
		"page", params.Page,
		"status", result.StatusCode,
		"duration_ms", time.Since(fetchStart).Milliseconds(),
	)
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
	u, err := url.Parse(base)
	if err != nil {
		u = &url.URL{Path: base}
	}
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
	if params.MaxKm > 0 {
		v.Set("km", "-1-"+strconv.Itoa(params.MaxKm))
	}
	if params.EngineMinCC > 0 {
		v.Set("engineval", strconv.Itoa(params.EngineMinCC)+"--1")
	}
	if params.MaxHand > 0 {
		v.Set("hand", "0-"+strconv.Itoa(params.MaxHand))
	}
	switch strings.ToLower(strings.TrimSpace(params.SellerFilter)) {
	case "private":
		v.Set("ownerID", "1")
	case "commercial", "dealer", "dealership":
		v.Set("ownerID", "2")
	}
	if params.PhotoOnly {
		v.Set("imgOnly", "1")
	}
	v.Set("Order", "1")
	if params.Page > 0 {
		v.Set("page", strconv.Itoa(params.Page))
	}

	u.RawQuery = v.Encode()
	return u.String()
}

// looksLikeGenericError detects generic nginx/CDN error pages that don't
// contain explicit bot-protection markers but are still anti-bot responses.
func looksLikeGenericError(body []byte) bool {
	h := strings.ToLower(string(body))
	return strings.Contains(h, "<title>400 bad request</title>") ||
		strings.Contains(h, "<title>403 forbidden</title>")
}

func looksLikeBotProtection(body []byte) bool {
	h := strings.ToLower(string(body))
	return strings.Contains(h, "perfdrive") ||
		strings.Contains(h, "shieldsquare") ||
		strings.Contains(h, "validate.perfdrive.com") ||
		strings.Contains(h, "are you for real") ||
		strings.Contains(h, "cf-browser-verification") ||
		strings.Contains(h, "captcha")
}
