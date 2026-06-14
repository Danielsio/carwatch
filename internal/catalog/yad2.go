package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
)

// CatalogResult holds manufacturers and models parsed from a Yad2 page.
type CatalogResult struct {
	Manufacturers map[int]Entry         // id -> Entry
	Models        map[int]map[int]Entry // mfr_id -> model_id -> Entry
}

// Yad2PageFetcher abstracts fetching a raw Yad2 HTML page body so the
// catalog loader can be tested without a real HTTP client.
type Yad2PageFetcher interface {
	FetchPage(ctx context.Context, url string) ([]byte, error)
}

const defaultCatalogURL = "https://www.yad2.co.il/vehicles/cars"

// FetchCatalogFromYad2 fetches the Yad2 cars page and extracts
// manufacturer/model entries from listings in __NEXT_DATA__.
func FetchCatalogFromYad2(ctx context.Context, fetcher Yad2PageFetcher) (*CatalogResult, error) {
	body, err := fetcher.FetchPage(ctx, defaultCatalogURL)
	if err != nil {
		return nil, fmt.Errorf("fetch yad2 page: %w", err)
	}
	return ParseCatalogFromHTML(body)
}

// ParseCatalogFromHTML extracts manufacturer/model catalog entries from
// Yad2 HTML containing a __NEXT_DATA__ script tag.
func ParseCatalogFromHTML(html []byte) (*CatalogResult, error) {
	data, err := yad2.ExtractNextDataJSON(bytes.NewReader(html))
	if err != nil {
		return nil, err
	}
	return parseCatalogFromNextData(data)
}

func parseCatalogFromNextData(data []byte) (*CatalogResult, error) {
	var nd struct {
		Props struct {
			PageProps struct {
				DehydratedState struct {
					Queries []struct {
						State struct {
							Data json.RawMessage `json:"data"`
						} `json:"state"`
					} `json:"queries"`
				} `json:"dehydratedState"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(data, &nd); err != nil {
		return nil, fmt.Errorf("unmarshal __NEXT_DATA__: %w", err)
	}

	result := &CatalogResult{
		Manufacturers: make(map[int]Entry),
		Models:        make(map[int]map[int]Entry),
	}

	listingKeys := []string{"private", "commercial", "platinum", "solo", "boost"}

	for _, q := range nd.Props.PageProps.DehydratedState.Queries {
		var bucket map[string]json.RawMessage
		if err := json.Unmarshal(q.State.Data, &bucket); err != nil {
			continue
		}

		for _, key := range listingKeys {
			raw, ok := bucket[key]
			if !ok || raw == nil || string(raw) == "null" {
				continue
			}
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				continue
			}
			for _, item := range items {
				extractCatalogEntry(item, result)
			}
		}

		// Legacy format
		var legacy struct {
			Data struct {
				Feed struct {
					FeedItems json.RawMessage `json:"feed_items"`
				} `json:"feed"`
			} `json:"data"`
		}
		if err := json.Unmarshal(q.State.Data, &legacy); err == nil &&
			legacy.Data.Feed.FeedItems != nil &&
			string(legacy.Data.Feed.FeedItems) != "null" {
			var items []json.RawMessage
			if json.Unmarshal(legacy.Data.Feed.FeedItems, &items) == nil {
				for _, item := range items {
					extractCatalogEntry(item, result)
				}
			}
		}
	}

	if len(result.Manufacturers) == 0 {
		return nil, fmt.Errorf("no manufacturer data found in page")
	}

	return result, nil
}

type catalogField struct {
	ID          int    `json:"id"`
	Text        string `json:"text"`
	EnglishText string `json:"english_text"`
	TextEng     string `json:"textEng"`
}

func (f catalogField) english() string {
	if f.EnglishText != "" {
		return f.EnglishText
	}
	return f.TextEng
}

func extractCatalogEntry(raw json.RawMessage, result *CatalogResult) {
	var item struct {
		Manufacturer catalogField `json:"manufacturer"`
		Model        catalogField `json:"model"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return
	}

	mfr := item.Manufacturer
	if mfr.ID == 0 {
		return
	}

	engName := mfr.english()
	if engName == "" {
		engName = mfr.Text
	}

	if _, exists := result.Manufacturers[mfr.ID]; !exists {
		result.Manufacturers[mfr.ID] = Entry{
			ID:     mfr.ID,
			Name:   engName,
			NameHe: mfr.Text,
		}
	}

	mdl := item.Model
	if mdl.ID == 0 {
		return
	}

	engModelName := mdl.english()
	if engModelName == "" {
		engModelName = mdl.Text
	}

	if result.Models[mfr.ID] == nil {
		result.Models[mfr.ID] = make(map[int]Entry)
	}
	if _, exists := result.Models[mfr.ID][mdl.ID]; !exists {
		result.Models[mfr.ID][mdl.ID] = Entry{
			ID:     mdl.ID,
			Name:   engModelName,
			NameHe: mdl.Text,
		}
	}
}

// NewHTTPPageFetcher wraps an io.Reader-returning function to implement
// Yad2PageFetcher. This is used to bridge the yad2 HTTPDoer interface.
type HTTPPageFetcher struct {
	GetPage func(ctx context.Context, url string) ([]byte, error)
}

func (f *HTTPPageFetcher) FetchPage(ctx context.Context, url string) ([]byte, error) {
	return f.GetPage(ctx, url)
}

// StaticPageFetcher returns fixed HTML content (for testing).
type StaticPageFetcher struct {
	HTML []byte
	Err  error
}

func (f *StaticPageFetcher) FetchPage(_ context.Context, _ string) ([]byte, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.HTML, nil
}

// NewYad2PageFetcherFromReader creates a Yad2PageFetcher from a reader (for testing).
func NewYad2PageFetcherFromReader(r io.Reader) Yad2PageFetcher {
	body, _ := io.ReadAll(r)
	return &StaticPageFetcher{HTML: body}
}

// NewYad2PageFetcherFromBytes creates a Yad2PageFetcher from raw bytes.
func NewYad2PageFetcherFromBytes(b []byte) Yad2PageFetcher {
	return &StaticPageFetcher{HTML: bytes.Clone(b)}
}
