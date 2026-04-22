package yad2

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

const challengeMarker = "Are you for real"

var nextDataRe = regexp.MustCompile(`(?s)<script\s+id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func ParseListingsPage(body io.Reader) ([]model.RawListing, error) {
	return ParseListingsPageWithLogger(body, nil)
}

func ParseListingsPageWithLogger(body io.Reader, logger *slog.Logger) ([]model.RawListing, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	html := string(raw)

	if strings.Contains(html, challengeMarker) {
		return nil, fmt.Errorf("yad2: %w", fetcher.ErrChallenge)
	}

	matches := nextDataRe.FindStringSubmatch(html)
	if len(matches) < 2 || matches[1] == "" {
		return nil, fmt.Errorf("__NEXT_DATA__ script tag not found")
	}

	return parseNextData([]byte(matches[1]), logger)
}

func parseNextData(data []byte, logger *slog.Logger) ([]model.RawListing, error) {
	var nextData nextDataEnvelope
	if err := json.Unmarshal(data, &nextData); err != nil {
		return nil, fmt.Errorf("unmarshal __NEXT_DATA__: %w", err)
	}

	items, err := extractItems(nextData)
	if err != nil {
		return nil, err
	}

	listings := make([]model.RawListing, 0, len(items))
	skipped := 0
	for _, item := range items {
		l, err := itemToListing(item)
		if err != nil {
			skipped++
			if logger != nil {
				logger.Warn("skipped feed item", "error", err)
			}
			continue
		}
		listings = append(listings, l)
	}

	if skipped > 0 && logger != nil {
		logger.Warn("skipped items during parse",
			"skipped", skipped,
			"total", len(items),
			"parsed", len(listings),
		)
	}

	return listings, nil
}

func extractItems(nd nextDataEnvelope) ([]json.RawMessage, error) {
	type feedProbe struct {
		Data struct {
			Feed struct {
				FeedItems json.RawMessage `json:"feed_items"`
			} `json:"feed"`
		} `json:"data"`
	}

	queries := nd.Props.PageProps.DehydratedState.Queries
	for _, q := range queries {
		var probe feedProbe
		if err := json.Unmarshal(q.State.Data, &probe); err != nil {
			continue
		}
		raw := probe.Data.Feed.FeedItems
		if raw == nil {
			continue
		}
		if string(raw) == "null" {
			return []json.RawMessage{}, nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			continue
		}
		return items, nil
	}
	return nil, fmt.Errorf("no feed items found in __NEXT_DATA__")
}

func itemToListing(raw json.RawMessage) (model.RawListing, error) {
	var item feedItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return model.RawListing{}, err
	}
	if item.Token == "" {
		return model.RawListing{}, fmt.Errorf("item has no token")
	}

	listing := model.RawListing{
		Token:            item.Token,
		Manufacturer:     textFromField(item.Manufacturer),
		ManufacturerID:   item.Manufacturer.ID,
		Model:            textFromField(item.Model),
		ModelID:          item.Model.ID,
		SubModel:         textFromField(item.SubModel),
		Year:         item.Year,
		Month:        item.Month,
		EngineVolume: item.EngineVolume,
		HorsePower:   item.HorsePower,
		EngineType:   textFromField(item.EngineType),
		GearBox:      textFromField(item.GearBox),
		Km:           item.Km,
		Hand:         item.Hand,
		Price:        item.Price,
		Description:  item.MetaData.Description,
		ImageURL:     item.MetaData.CoverImage,
		PageLink:     fmt.Sprintf("https://www.yad2.co.il/item/%s", item.Token),
	}

	if item.Address.City.Text != "" {
		listing.City = item.Address.City.Text
	}
	if item.Address.Area.Text != "" {
		listing.Area = item.Address.Area.Text
	}

	if item.Dates.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, item.Dates.CreatedAt); err == nil {
			listing.CreatedAt = t
		}
	}
	if item.Dates.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, item.Dates.UpdatedAt); err == nil {
			listing.UpdatedAt = t
		}
	}

	return listing, nil
}

func textFromField(f field) string {
	if f.EnglishText != "" {
		return f.EnglishText
	}
	return f.Text
}

type nextDataEnvelope struct {
	Props struct {
		PageProps struct {
			DehydratedState struct {
				Queries []queryEntry `json:"queries"`
			} `json:"dehydratedState"`
		} `json:"pageProps"`
	} `json:"props"`
}

type queryEntry struct {
	State struct {
		Data json.RawMessage `json:"data"`
	} `json:"state"`
}

type feedItem struct {
	Token        string  `json:"token"`
	Manufacturer field   `json:"manufacturer"`
	Model        field   `json:"model"`
	SubModel     field   `json:"subModel"`
	Year         int     `json:"year_of_production"`
	Month        int     `json:"month_of_production"`
	EngineVolume float64 `json:"engine_volume"`
	HorsePower   int     `json:"horsePower"`
	EngineType   field   `json:"engineType"`
	GearBox      field   `json:"gearBox"`
	Km           int     `json:"km"`
	Hand         int     `json:"hand"`
	Price        int     `json:"price"`
	Address      struct {
		City field `json:"city"`
		Area field `json:"area"`
	} `json:"address"`
	MetaData struct {
		CoverImage  string `json:"coverImage"`
		Description string `json:"description"`
	} `json:"metaData"`
	Dates struct {
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	} `json:"dates"`
}

type field struct {
	Text        string `json:"text"`
	EnglishText string `json:"english_text"`
	ID          int    `json:"id"`
}
