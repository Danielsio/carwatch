package yad2

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var itemNextDataRe = regexp.MustCompile(`(?is)<script[^>]*\bid=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)

// ItemDetails holds enrichment data parsed from an individual listing page.
type ItemDetails struct {
	Km       int
	ImageURL string
	City     string
	Area     string
}

// ParseItemPage extracts listing details (primarily km) from a Yad2 item page.
func ParseItemPage(body io.Reader) (ItemDetails, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return ItemDetails{}, fmt.Errorf("read item page body: %w", err)
	}
	html := string(raw)

	if strings.Contains(html, challengeMarker) {
		return ItemDetails{}, fmt.Errorf("yad2 item: challenge page")
	}

	matches := itemNextDataRe.FindStringSubmatch(html)
	if len(matches) < 2 || matches[1] == "" {
		return ItemDetails{}, fmt.Errorf("item page: __NEXT_DATA__ not found")
	}

	return parseItemNextData([]byte(matches[1]))
}

func parseItemNextData(data []byte) (ItemDetails, error) {
	// Try pageProps.itemData first (common item page structure).
	var envelope struct {
		Props struct {
			PageProps struct {
				ItemData *itemPageData `json:"itemData"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ItemDetails{}, fmt.Errorf("unmarshal item __NEXT_DATA__: %w", err)
	}

	if envelope.Props.PageProps.ItemData != nil {
		d := *envelope.Props.PageProps.ItemData
		if details, ok := detailsFromPageData(d); ok {
			return details, nil
		}
	}

	// Fallback: search dehydratedState queries for km field.
	var dehydrated struct {
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
	if err := json.Unmarshal(data, &dehydrated); err == nil {
		for _, q := range dehydrated.Props.PageProps.DehydratedState.Queries {
			if q.State.Data == nil {
				continue
			}
			var item itemPageData
			if json.Unmarshal(q.State.Data, &item) == nil {
				if details, ok := detailsFromPageData(item); ok {
					return details, nil
				}
			}
			var wrapper map[string]json.RawMessage
			if json.Unmarshal(q.State.Data, &wrapper) == nil {
				for _, v := range wrapper {
					var nested itemPageData
					if json.Unmarshal(v, &nested) == nil {
						if details, ok := detailsFromPageData(nested); ok {
							return details, nil
						}
					}
				}
			}
		}
	}

	return ItemDetails{}, fmt.Errorf("no enrichment data found in item page")
}

type itemPageData struct {
	Km         int    `json:"km"`
	Kilometer  int    `json:"kilometer"`
	CoverImage string `json:"coverImage"`
	Address    struct {
		City struct {
			Text    string `json:"text"`
			TextEng string `json:"textEng"`
		} `json:"city"`
		Area struct {
			Text    string `json:"text"`
			TextEng string `json:"textEng"`
		} `json:"area"`
	} `json:"address"`
}

func detailsFromPageData(d itemPageData) (ItemDetails, bool) {
	details := ItemDetails{
		Km:       effectiveKm(d),
		ImageURL: d.CoverImage,
		City:     firstNonEmpty(d.Address.City.TextEng, d.Address.City.Text),
		Area:     firstNonEmpty(d.Address.Area.TextEng, d.Address.Area.Text),
	}
	return details, details.Km > 0 || details.ImageURL != "" || details.City != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func effectiveKm(d itemPageData) int {
	if d.Km > 0 {
		return d.Km
	}
	return d.Kilometer
}
