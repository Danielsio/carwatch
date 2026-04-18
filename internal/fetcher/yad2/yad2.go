package yad2

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

const (
	defaultBaseURL  = "https://www.yad2.co.il/vehicles/cars"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

type Yad2Fetcher struct {
	client  *Client
	baseURL string
	logger  *slog.Logger
}

func NewFetcher(userAgents []string, proxy string, logger *slog.Logger) (*Yad2Fetcher, error) {
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	return &Yad2Fetcher{client: client, baseURL: defaultBaseURL, logger: logger}, nil
}

func (f *Yad2Fetcher) Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error) {
	reqURL := buildURL(f.baseURL, params)
	f.logger.Info("fetching listings", "url", reqURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var reader io.Reader = io.LimitReader(resp.Body, maxResponseSize)
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	listings, err := ParseListingsPage(reader)
	if err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}

	f.logger.Info("fetched listings", "count", len(listings))
	return listings, nil
}

func buildURL(base string, params config.SourceParams) string {
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

	u.RawQuery = v.Encode()
	return u.String()
}
