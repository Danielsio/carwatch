package yad2

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ItemDetails holds enrichment data parsed from an individual listing page.
type ItemDetails struct {
	Km                int
	ImageURL          string
	City              string
	Area              string
	OriginalOwnership string
	CurrentOwnership  string
}

// ParseItemPage extracts listing details (primarily km) from a Yad2 item page.
func ParseItemPage(body io.Reader) (ItemDetails, error) {
	data, err := ExtractNextDataJSON(body)
	if err != nil {
		return ItemDetails{}, err
	}
	return parseItemNextData(data)
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

type ownershipField struct {
	Text    string `json:"text"`
	TextEng string `json:"textEng"`
	ID      int    `json:"id"`
}

type itemPageData struct {
	Km         int      `json:"km"`
	Kilometer  int      `json:"kilometer"`
	CoverImage string   `json:"coverImage"`
	CoverImg   string   `json:"cover_image"`
	Images     []string `json:"images"`
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
	CurrentOwnership  *ownershipField `json:"currentOwnership"`
	OriginalOwnership *ownershipField `json:"originalOwnership"`
	Ownership         *ownershipField `json:"ownership"`
	PreviousOwnership *ownershipField `json:"previousOwnership"`
}

func detailsFromPageData(d itemPageData) (ItemDetails, bool) {
	details := ItemDetails{
		Km:       effectiveKm(d),
		ImageURL: resolveItemImageURL(d),
		City:     firstNonEmpty(d.Address.City.TextEng, d.Address.City.Text),
		Area:     firstNonEmpty(d.Address.Area.TextEng, d.Address.Area.Text),
	}

	// Extract original ownership: try OriginalOwnership, PreviousOwnership, Ownership.
	for _, f := range []*ownershipField{d.OriginalOwnership, d.PreviousOwnership, d.Ownership} {
		if f == nil {
			continue
		}
		if v := normalizeOwnership(firstNonEmpty(f.Text, f.TextEng)); v != "" {
			details.OriginalOwnership = v
			break
		}
		if v := normalizeOwnership(firstNonEmpty(f.TextEng, f.Text)); v != "" {
			details.OriginalOwnership = v
			break
		}
	}

	// Extract current ownership from CurrentOwnership field.
	if d.CurrentOwnership != nil {
		if v := normalizeOwnership(firstNonEmpty(d.CurrentOwnership.Text, d.CurrentOwnership.TextEng)); v != "" {
			details.CurrentOwnership = v
		} else if v := normalizeOwnership(firstNonEmpty(d.CurrentOwnership.TextEng, d.CurrentOwnership.Text)); v != "" {
			details.CurrentOwnership = v
		}
	}

	return details, details.Km > 0 || details.ImageURL != "" || details.City != "" || details.Area != "" ||
		details.OriginalOwnership != "" || details.CurrentOwnership != ""
}

// normalizeOwnership canonicalizes a Hebrew or English ownership string to one
// of "private", "lease", or "rental". Returns "" for unrecognized input.
func normalizeOwnership(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return ""
	}
	lower := strings.ToLower(t)
	switch {
	case strings.Contains(t, "פרטי"):
		return "private"
	case strings.Contains(t, "ליסינג") || strings.Contains(lower, "leasing"):
		return "lease"
	case strings.Contains(t, "השכרה") || strings.Contains(lower, "rental"):
		return "rental"
	case lower == "private":
		return "private"
	case lower == "lease" || lower == "leasing":
		return "lease"
	case lower == "rental":
		return "rental"
	default:
		return ""
	}
}

// resolveItemImageURL checks multiple image field locations in the item page
// data, returning the first non-empty URL found.
func resolveItemImageURL(d itemPageData) string {
	if d.CoverImage != "" {
		return d.CoverImage
	}
	if d.CoverImg != "" {
		return d.CoverImg
	}
	if len(d.Images) > 0 && d.Images[0] != "" {
		return d.Images[0]
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
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
