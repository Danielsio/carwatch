package yad2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/bodytype"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

const challengeMarker = "Are you for real"

var nextDataRe = regexp.MustCompile(`(?is)<script[^>]*\bid=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)

// subModelHPRe extracts horsepower from sub-model text like "(165 כ״ס)".
var subModelHPRe = regexp.MustCompile(`\((\d+)\s*כ״ס\)`)

// subModelGearRe detects automatic transmission markers in sub-model text.
var subModelGearRe = regexp.MustCompile(`אוט[׳']`)

// ExtractNextDataJSON reads an HTML page, checks for a bot challenge, and
// returns the raw JSON content of the __NEXT_DATA__ script tag.
func ExtractNextDataJSON(body io.Reader) ([]byte, error) {
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

	return []byte(matches[1]), nil
}

func ParseListingsPage(body io.Reader) ([]model.RawListing, error) {
	return ParseListingsPageWithLogger(body, nil)
}

func ParseListingsPageWithLogger(body io.Reader, logger *slog.Logger) ([]model.RawListing, error) {
	data, err := ExtractNextDataJSON(body)
	if err != nil {
		return nil, err
	}
	return parseNextData(data, logger)
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

	if logger != nil && len(items) > 0 && logger.Enabled(context.Background(), slog.LevelDebug) {
		var probe feedItem
		if err := json.Unmarshal(items[0].raw, &probe); err == nil {
			logger.Debug("yad2 feed item summary",
				"token", probe.Token,
				"km", probe.Km,
				"has_city", strings.TrimSpace(textFromField(probe.Address.City)) != "",
				"has_area", strings.TrimSpace(textFromField(probe.Address.Area)) != "",
				"price", probe.Price,
			)
		}
		logger.Debug("raw feed item sample", "json", string(items[0].raw))
	}

	return buildListings(items, logger), nil
}

// buildListings converts tagged feed items into de-duplicated RawListings via
// itemToListing. Shared by the HTML (__NEXT_DATA__) and gw JSON feed parsers.
func buildListings(items []feedTaggedItem, logger *slog.Logger) []model.RawListing {
	listings := make([]model.RawListing, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	skipped := 0
	for _, tagged := range items {
		l, err := itemToListing(tagged.raw, tagged.commercial, logger)
		if err != nil {
			skipped++
			if logger != nil {
				logger.Warn("skipped feed item", "error", err)
			}
			continue
		}
		if _, dup := seen[l.Token]; dup {
			continue
		}
		seen[l.Token] = struct{}{}
		listings = append(listings, l)
	}

	if skipped > 0 && logger != nil {
		logger.Warn("skipped items during parse",
			"skipped", skipped,
			"total", len(items),
			"parsed", len(listings),
		)
	}

	return listings
}

// gwFeedResponse is the shape of Yad2's gw JSON feed API response. FeedItems is
// a pointer so we can distinguish an empty feed ([] — a valid "no results")
// from a missing/changed shape (nil — treat as an error so Fetch falls back).
type gwFeedResponse struct {
	Data struct {
		Feed struct {
			FeedItems *[]json.RawMessage `json:"feed_items"`
		} `json:"feed"`
	} `json:"data"`
}

// ParseGwFeed parses Yad2's gw feed API response (data.feed.feed_items). Those
// items carry the rich fields (km, city, bodyType, gearBox) that the HTML
// page's __NEXT_DATA__ no longer includes, so this lets us skip most per-item
// enrichment. Field mapping is shared with the HTML feed via itemToListing.
func ParseGwFeed(body io.Reader, logger *slog.Logger) ([]model.RawListing, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read gw feed body: %w", err)
	}
	if looksLikeBotProtection(raw) {
		return nil, fmt.Errorf("yad2 gw feed: %w", fetcher.ErrChallenge)
	}

	var resp gwFeedResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal gw feed: %w", err)
	}
	if resp.Data.Feed.FeedItems == nil {
		return nil, fmt.Errorf("gw feed missing data.feed.feed_items")
	}
	feedItems := *resp.Data.Feed.FeedItems

	items := make([]feedTaggedItem, 0, len(feedItems))
	for _, it := range feedItems {
		items = append(items, feedTaggedItem{raw: it, commercial: nil})
	}
	listings := buildListings(items, logger)

	if logger != nil {
		withKm, withBody := 0, 0
		for i := range listings {
			if listings[i].Km > 0 {
				withKm++
			}
			if listings[i].BodyType != "" {
				withBody++
			}
		}
		// Confirms the gw feed's richness in prod: with_km/with_body_type ≈
		// listings means per-item enrichment is largely unnecessary.
		logger.Debug("parsed gw feed",
			"items", len(feedItems), "listings", len(listings),
			"with_km", withKm, "with_body_type", withBody)
	}
	return listings, nil
}

var listingKeys = []string{"private", "commercial", "platinum", "solo", "boost"}

type feedTaggedItem struct {
	raw        json.RawMessage
	commercial *bool // nil: legacy feed (unknown); false: private; true: dealer/commercial buckets
}

// listingKeyCommercial maps Yad2 __NEXT_DATA__ bucket keys to seller type.
func listingKeyCommercial(key string) *bool {
	switch key {
	case "private":
		v := false
		return &v
	case "commercial", "platinum", "solo", "boost":
		v := true
		return &v
	default:
		return nil
	}
}

func extractItems(nd nextDataEnvelope) ([]feedTaggedItem, error) {
	queries := nd.Props.PageProps.DehydratedState.Queries
	for _, q := range queries {
		var bucket map[string]json.RawMessage
		if err := json.Unmarshal(q.State.Data, &bucket); err != nil {
			continue
		}

		var all []feedTaggedItem
		found := false
		for _, key := range listingKeys {
			raw, ok := bucket[key]
			if !ok || raw == nil || string(raw) == "null" {
				continue
			}
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				continue
			}
			found = true
			tag := listingKeyCommercial(key)
			for _, item := range items {
				all = append(all, feedTaggedItem{raw: item, commercial: tag})
			}
		}
		if found {
			return all, nil
		}

		// Legacy format: data.feed.feed_items.
		var legacy struct {
			Data struct {
				Feed struct {
					FeedItems json.RawMessage `json:"feed_items"`
				} `json:"feed"`
			} `json:"data"`
		}
		if err := json.Unmarshal(q.State.Data, &legacy); err == nil && legacy.Data.Feed.FeedItems != nil {
			if string(legacy.Data.Feed.FeedItems) == "null" {
				return []feedTaggedItem{}, nil
			}
			var items []json.RawMessage
			if json.Unmarshal(legacy.Data.Feed.FeedItems, &items) == nil {
				out := make([]feedTaggedItem, 0, len(items))
				for _, item := range items {
					out = append(out, feedTaggedItem{raw: item, commercial: nil})
				}
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("no feed items found in __NEXT_DATA__")
}

func itemToListing(raw json.RawMessage, commercial *bool, logger *slog.Logger) (model.RawListing, error) {
	var item feedItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return model.RawListing{}, err
	}
	if item.Token == "" {
		return model.RawListing{}, fmt.Errorf("item has no token")
	}

	year := item.Year
	if year == 0 {
		year = item.VehicleDates.YearOfProduction
	}
	engineVol := item.EngineVolume
	if engineVol == 0 {
		engineVol = item.EngineVolumeNew
	}

	imgURL := resolveImageURL(item)
	if imgURL == "" && logger != nil {
		logger.Warn("feed item has no image URL",
			"token", item.Token,
			"manufacturer", textFromField(item.Manufacturer),
			"model", textFromField(item.Model),
		)
	}

	listing := model.RawListing{
		Token:              item.Token,
		Manufacturer:       textFromField(item.Manufacturer),
		ManufacturerID:     item.Manufacturer.ID,
		ManufacturerNameHe: item.Manufacturer.Text,
		Model:              textFromField(item.Model),
		ModelID:            item.Model.ID,
		ModelNameHe:        item.Model.Text,
		SubModel:           textFromField(item.SubModel),
		SubModelID:         item.SubModel.ID,
		Year:               year,
		Month:              item.Month,
		EngineVolume:       engineVol,
		HorsePower:         item.HorsePower,
		EngineType:         textFromField(item.EngineType),
		GearBox:            textFromField(item.GearBox),
		Km:                 item.Km,
		Hand:               parseHand(item.Hand),
		Price:              item.Price,
		Description:        item.MetaData.Description,
		ImageURL:           imgURL,
		PageLink:           fmt.Sprintf("https://www.yad2.co.il/vehicles/item/%s", item.Token),
	}

	// Extract HP and gearbox from sub-model text when the feed
	// doesn't provide them as separate fields.
	subModelText := textFromField(item.SubModel)
	if listing.HorsePower == 0 {
		if m := subModelHPRe.FindStringSubmatch(subModelText); len(m) == 2 {
			if hp, err := strconv.Atoi(m[1]); err == nil {
				listing.HorsePower = hp
			}
		}
	}
	if listing.GearBox == "" && subModelGearRe.MatchString(subModelText) {
		listing.GearBox = "אוטומט"
	}

	// Body type comes ONLY from Yad2's structured bodyType field (id + label).
	// We deliberately do not guess from the free-text sub-model name — that gave
	// wrong/partial results (it only classified trims that happened to contain a
	// body keyword). When Yad2 provides no bodyType, body_type stays empty.
	listing.BodyType = bodytype.FromYad2(item.BodyType.ID, textFromField(item.BodyType), item.BodyType.Text)

	listing.City = textFromField(item.Address.City)
	listing.Area = textFromField(item.Address.Area)

	// posted_at must be the listing's ORIGINAL publish date ("פורסם ב" on the
	// item page). In the Yad2 feed that is dates.createdAt, which is unique per
	// listing. Do NOT fall back to dates.updatedAt / rebouncedAt: those carry a
	// single feed-wide "refreshed" timestamp (what drives the "חדש היום" badge),
	// so using them would make every listing look like it was posted today.
	createdAt := item.Dates.CreatedAt
	if createdAt == "" {
		createdAt = item.VehicleDates.CreatedAt
	}
	updatedAt := item.Dates.UpdatedAt
	if updatedAt == "" {
		updatedAt = item.VehicleDates.UpdatedAt
	}
	if createdAt != "" {
		listing.CreatedAt = parseFlexTime(createdAt)
		if listing.CreatedAt.IsZero() && logger != nil {
			// A non-empty value we cannot parse means Yad2 changed its date
			// format. Surface it loudly: silently dropping it makes posted_at
			// NULL and the UI falls back to first-seen ("today").
			logger.Warn("yad2: unparseable listing created-at; posted_at will be empty",
				"token", item.Token, "raw_created_at", createdAt)
		}
	}
	if updatedAt != "" {
		listing.UpdatedAt = parseFlexTime(updatedAt)
	}
	listing.Commercial = commercial
	return listing, nil
}

// resolveImageURL checks multiple image field locations in the feed item,
// returning the first non-empty URL found. This handles Yad2 API format
// changes where the image may appear in different fields.
func resolveImageURL(item feedItem) string {
	if item.MetaData.CoverImage != "" {
		return item.MetaData.CoverImage
	}
	if item.MetaData.CoverImg != "" {
		return item.MetaData.CoverImg
	}
	if item.CoverImageTop != "" {
		return item.CoverImageTop
	}
	if item.CoverImgTop != "" {
		return item.CoverImgTop
	}
	if len(item.Images) > 0 && item.Images[0] != "" {
		return item.Images[0]
	}
	return ""
}

func parseHand(raw json.RawMessage) int {
	if raw == nil {
		return 0
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var f field
	if json.Unmarshal(raw, &f) == nil {
		return f.ID
	}
	return 0
}

var israelTZ = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// parseFlexTime parses the timestamp formats Yad2 has been observed to emit.
// Yad2's feed dates are zone-less local time (e.g. "2025-02-09T10:31:37"), but
// it has historically drifted between "Z"/offset-suffixed and fractional-second
// variants, so we accept all of them rather than silently dropping the date.
func parseFlexTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// Zone-bearing formats ("...Z" or "...+02:00"), with or without fractional
	// seconds. RFC3339Nano covers the fractional case; RFC3339 the rest.
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	// Zone-less formats — interpret as Israel local time, matching the feed.
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, s, israelTZ); err == nil {
			return t
		}
	}
	return time.Time{}
}

func textFromField(f field) string {
	if f.EnglishText != "" {
		return f.EnglishText
	}
	if f.TextEng != "" {
		return f.TextEng
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
	Token           string          `json:"token"`
	Manufacturer    field           `json:"manufacturer"`
	Model           field           `json:"model"`
	SubModel        field           `json:"subModel"`
	Year            int             `json:"year_of_production"`
	Month           int             `json:"month_of_production"`
	EngineVolume    float64         `json:"engine_volume"`
	EngineVolumeNew float64         `json:"engineVolume"`
	HorsePower      int             `json:"horsePower"`
	EngineType      field           `json:"engineType"`
	GearBox         field           `json:"gearBox"`
	BodyType        field           `json:"bodyType"`
	Km              int             `json:"km"`
	Hand            json.RawMessage `json:"hand"`
	Price           int             `json:"price"`
	Address         struct {
		City field `json:"city"`
		Area field `json:"area"`
	} `json:"address"`
	MetaData struct {
		CoverImage  string `json:"coverImage"`
		CoverImg    string `json:"cover_image"`
		Description string `json:"description"`
	} `json:"metaData"`
	// Top-level image fields as fallback when metaData.coverImage is absent.
	Images        []string `json:"images"`
	CoverImageTop string   `json:"coverImage"`
	CoverImgTop   string   `json:"cover_image"`
	Dates         struct {
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	} `json:"dates"`
	VehicleDates struct {
		CreatedAt        string `json:"createdAt"`
		UpdatedAt        string `json:"updatedAt"`
		YearOfProduction int    `json:"yearOfProduction"`
	} `json:"vehicleDates"`
}

type field struct {
	Text        string `json:"text"`
	EnglishText string `json:"english_text"`
	TextEng     string `json:"textEng"`
	ID          int    `json:"id"`
}
