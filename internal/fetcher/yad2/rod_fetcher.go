package yad2

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"

	"github.com/dsionov/carwatch/internal/model"
)

// RodFetcher uses a real Chrome browser via the DevTools Protocol to fetch
// Yad2 listings. Chrome's genuine TLS fingerprint and JS execution bypass
// Radware's anti-bot without needing proxies or stealth HTTP clients.
type RodFetcher struct {
	mu      sync.Mutex
	browser *rod.Browser
	binPath string
	logger  *slog.Logger
}

// NewRodFetcher creates a fetcher backed by a headless Chrome instance.
// binPath is the path to the Chrome/Chromium binary.
func NewRodFetcher(binPath string, logger *slog.Logger) (*RodFetcher, error) {
	f := &RodFetcher{binPath: binPath, logger: logger}
	if err := f.ensureBrowser(); err != nil {
		return nil, fmt.Errorf("rod: launch browser: %w", err)
	}
	return f, nil
}

func (f *RodFetcher) ensureBrowser() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.browser != nil {
		return nil
	}

	l := launcher.New().Bin(f.binPath).
		Leakless(false).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox").
		Set("disable-dev-shm-usage").
		Set("headless", "new")

	u, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch chrome: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect to chrome: %w", err)
	}

	f.browser = browser
	f.logger.Info("rod browser launched", "bin", f.binPath)
	return nil
}

// Fetch navigates Chrome to the Yad2 search URL, waits for __NEXT_DATA__,
// and parses the listings using the shared parser.
func (f *RodFetcher) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	if err := f.ensureBrowser(); err != nil {
		return nil, err
	}

	reqURL := buildURL(defaultBaseURL, params)
	start := time.Now()
	f.logger.Debug("rod: fetching page", "url", reqURL)

	page, err := stealth.Page(f.browser)
	if err != nil {
		f.browser = nil
		return nil, fmt.Errorf("rod: create stealth page: %w", err)
	}
	defer page.MustClose()

	if err := page.Context(ctx).Navigate(reqURL); err != nil {
		return nil, fmt.Errorf("rod: navigate: %w", err)
	}

	if err := f.waitForContent(ctx, page); err != nil {
		return nil, err
	}

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("rod: get HTML: %w", err)
	}

	listings, err := ParseListingsPageWithLogger(bytes.NewReader([]byte(html)), f.logger)
	if err != nil {
		return nil, fmt.Errorf("rod: %w", err)
	}

	f.logger.Debug("rod: fetched listings",
		"count", len(listings),
		"manufacturer", params.Manufacturer,
		"model", params.Model,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return listings, nil
}

// FetchItem navigates Chrome to an individual listing page and extracts
// enrichment details (km, city, bodyType, etc.).
func (f *RodFetcher) FetchItem(ctx context.Context, token string) (ItemDetails, error) {
	if err := f.ensureBrowser(); err != nil {
		return ItemDetails{}, err
	}

	itemURL := "https://www.yad2.co.il/vehicles/item/" + token
	f.logger.Debug("rod: fetching item page", "token", token)

	page, err := stealth.Page(f.browser)
	if err != nil {
		f.browser = nil
		return ItemDetails{}, fmt.Errorf("rod: create stealth page: %w", err)
	}
	defer page.MustClose()

	if err := page.Context(ctx).Navigate(itemURL); err != nil {
		return ItemDetails{}, fmt.Errorf("rod: navigate item %s: %w", token, err)
	}

	if err := f.waitForContent(ctx, page); err != nil {
		return ItemDetails{}, fmt.Errorf("rod: item %s: %w", token, err)
	}

	html, err := page.HTML()
	if err != nil {
		return ItemDetails{}, fmt.Errorf("rod: get item HTML: %w", err)
	}

	details, err := ParseItemPage(bytes.NewReader([]byte(html)))
	if err != nil {
		return ItemDetails{}, fmt.Errorf("rod: parse item %s: %w", token, err)
	}

	return details, nil
}

func (f *RodFetcher) waitForContent(ctx context.Context, page *rod.Page) error {
	deadline := time.After(15 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for page content")
		case <-tick.C:
			title, err := page.Eval(`() => document.title`)
			if err != nil {
				continue
			}
			t := title.Value.Str()
			if strings.Contains(strings.ToLower(t), "captcha") ||
				strings.Contains(strings.ToLower(t), "shieldsquare") {
				continue
			}
			if t != "" && !strings.Contains(strings.ToLower(t), "radware") {
				return nil
			}
		}
	}
}

// FetchRawPage fetches a URL using Chrome and returns the raw HTML body.
func (f *RodFetcher) FetchRawPage(ctx context.Context, rawURL string) ([]byte, error) {
	if err := f.ensureBrowser(); err != nil {
		return nil, err
	}

	page, err := stealth.Page(f.browser)
	if err != nil {
		f.browser = nil
		return nil, fmt.Errorf("rod: create page: %w", err)
	}
	defer page.MustClose()

	if err := page.Context(ctx).Navigate(rawURL); err != nil {
		return nil, fmt.Errorf("rod: navigate: %w", err)
	}

	if err := f.waitForContent(ctx, page); err != nil {
		return nil, err
	}

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("rod: get HTML: %w", err)
	}

	return []byte(html), nil
}

// Close shuts down the Chrome browser.
func (f *RodFetcher) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.browser != nil {
		_ = f.browser.Close()
		f.browser = nil
	}
}
