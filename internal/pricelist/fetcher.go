package pricelist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var basePriceLabelRe = regexp.MustCompile(`(?i)מחיר\s+בסיס[^₪]*₪\s*([\d,]+)`)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	},
}

func fetch(ctx context.Context, subModelID, year int) fetchResult {
	return fetchGoHTTP(ctx, subModelID, year)
}

func fetchGoHTTP(ctx context.Context, subModelID, year int) fetchResult {
	url := priceListURL(subModelID, year)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		return fetchResult{Error: fmt.Sprintf("create request: %v", err)}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "he-IL,he;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fetchResult{Error: fmt.Sprintf("http get: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fetchResult{Error: fmt.Sprintf("http status %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return fetchResult{Error: fmt.Sprintf("read body: %v", err)}
	}

	html := string(body)
	return extractPriceFromHTML(html)
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
