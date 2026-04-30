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
	Km int
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
		if km := effectiveKm(*envelope.Props.PageProps.ItemData); km > 0 {
			return ItemDetails{Km: km}, nil
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
				if km := effectiveKm(item); km > 0 {
					return ItemDetails{Km: km}, nil
				}
			}
			var wrapper map[string]json.RawMessage
			if json.Unmarshal(q.State.Data, &wrapper) == nil {
				for _, v := range wrapper {
					var nested itemPageData
					if json.Unmarshal(v, &nested) == nil {
						if km := effectiveKm(nested); km > 0 {
							return ItemDetails{Km: km}, nil
						}
					}
				}
			}
		}
	}

	return ItemDetails{}, fmt.Errorf("km not found in item page data")
}

type itemPageData struct {
	Km        int `json:"km"`
	Kilometer int `json:"kilometer"`
}

func effectiveKm(d itemPageData) int {
	if d.Km > 0 {
		return d.Km
	}
	return d.Kilometer
}
