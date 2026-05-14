package pricelist

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var basePriceLabelRe = regexp.MustCompile(`(?i)מחיר\s+בסיס[^₪]*₪\s*([\d,]+)`)

// HTTPResult mirrors yad2.HTTPResult so the pricelist package does not
// depend on the yad2 package.
type HTTPResult struct {
	Body       []byte
	StatusCode int
}

// HTTPDoer abstracts HTTP GET requests so the pricelist fetcher can use
// the same azuretls stealth client that the Yad2 listing scraper uses.
type HTTPDoer interface {
	Get(ctx context.Context, url string) (*HTTPResult, error)
}

func fetch(ctx context.Context, client HTTPDoer, subModelID, year int) fetchResult {
	url := priceListURL(subModelID, year)

	start := time.Now()
	res, err := client.Get(ctx, url)
	if err != nil {
		return fetchResult{Error: fmt.Sprintf("http get (%s): %v", time.Since(start).Round(time.Millisecond), err)}
	}

	if res.StatusCode != 200 {
		return fetchResult{Error: fmt.Sprintf("http status %d (url=%s, took=%s)", res.StatusCode, url, time.Since(start).Round(time.Millisecond))}
	}

	html := string(res.Body)
	result := extractPriceFromHTML(html)
	if result.Error != "" {
		result.Error = fmt.Sprintf("%s (url=%s, body_len=%d, took=%s)", result.Error, url, len(res.Body), time.Since(start).Round(time.Millisecond))
	}
	return result
}

func extractPriceFromHTML(html string) fetchResult {
	if m := basePriceLabelRe.FindStringSubmatch(html); len(m) > 1 {
		if price := parsePrice(m[1]); price > 1000 {
			return fetchResult{BasePrice: price}
		}
	}

	if idx := strings.Index(html, `"basePrice"`); idx >= 0 {
		start := strings.LastIndex(html[:idx], "{")
		if start >= 0 {
			end := strings.Index(html[idx:], "}")
			if end >= 0 {
				snippet := html[start : idx+end+1]
				var obj map[string]interface{}
				if json.Unmarshal([]byte(snippet), &obj) == nil {
					if bp, ok := obj["basePrice"].(float64); ok && bp > 1000 {
						return fetchResult{BasePrice: int(bp)}
					}
				}
			}
		}
	}

	return fetchResult{Error: "no price found in HTML"}
}

func parsePrice(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	v, _ := strconv.Atoi(s)
	return v
}
