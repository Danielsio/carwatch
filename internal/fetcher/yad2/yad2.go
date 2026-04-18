package yad2

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

const baseURL = "https://www.yad2.co.il/vehicles/cars"

type Yad2Fetcher struct {
	client *Client
	logger *slog.Logger
}

func NewFetcher(userAgents []string, proxy string, logger *slog.Logger) (*Yad2Fetcher, error) {
	client, err := NewClient(userAgents, proxy)
	if err != nil {
		return nil, err
	}
	return &Yad2Fetcher{client: client, logger: logger}, nil
}

func (f *Yad2Fetcher) Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error) {
	url := buildURL(params)
	f.logger.Info("fetching listings", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(resp.Body)
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

func buildURL(params config.SourceParams) string {
	u := baseURL + "?"
	parts := make([]string, 0, 6)

	if params.Manufacturer > 0 {
		parts = append(parts, "manufacturer="+strconv.Itoa(params.Manufacturer))
	}
	if params.Model > 0 {
		parts = append(parts, "model="+strconv.Itoa(params.Model))
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
		parts = append(parts, "year="+strconv.Itoa(yearMin)+"-"+strconv.Itoa(yearMax))
	}
	if params.PriceMin > 0 || params.PriceMax > 0 {
		priceMin := params.PriceMin
		priceMax := params.PriceMax
		if priceMax == 0 {
			priceMax = 9999999
		}
		parts = append(parts, "price="+strconv.Itoa(priceMin)+"-"+strconv.Itoa(priceMax))
	}
	parts = append(parts, "Order=1")

	return u + strings.Join(parts, "&")
}
