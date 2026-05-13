package pricelist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var priceRe = regexp.MustCompile(`₪\s*([\d,]+)`)
var basePriceLabelRe = regexp.MustCompile(`(?i)מחיר\s+בסיס[^₪]*₪\s*([\d,]+)`)

// fetch tries Go HTTP first, then falls back to the Python scraper.
func fetch(ctx context.Context, subModelID, year int, logger *slog.Logger) fetchResult {
	result := fetchGoHTTP(ctx, subModelID, year, logger)
	if result.BasePrice > 0 {
		return result
	}

	result = fetchPythonScraper(ctx, subModelID, year, logger)
	return result
}

// fetchGoHTTP attempts to fetch the price list page with a plain HTTP client
// and extract the base price from the HTML (works if the page is server-rendered
// or embeds data in __NEXT_DATA__).
func fetchGoHTTP(ctx context.Context, subModelID, year int, logger *slog.Logger) fetchResult {
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

	resp, err := http.DefaultClient.Do(req)
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
	// Strategy 1: look for "מחיר בסיס" label followed by a price
	if m := basePriceLabelRe.FindStringSubmatch(html); len(m) > 1 {
		if price := parsePrice(m[1]); price > 1000 {
			return fetchResult{BasePrice: price}
		}
	}

	// Strategy 2: look for __NEXT_DATA__ JSON with price info
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

	// Strategy 3: find all ₪ prices and pick the largest > 1000
	matches := priceRe.FindAllStringSubmatch(html, -1)
	var best int
	for _, m := range matches {
		if p := parsePrice(m[1]); p > 1000 && p > best {
			best = p
		}
	}
	if best > 0 {
		return fetchResult{BasePrice: best}
	}

	return fetchResult{Error: "no price found in HTML"}
}

func parsePrice(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	v, _ := strconv.Atoi(s)
	return v
}

// fetchPythonScraper shells out to the Python Yad2 price scraper as a fallback.
func fetchPythonScraper(ctx context.Context, subModelID, year int, logger *slog.Logger) fetchResult {
	python, err := exec.LookPath("python3")
	if err != nil {
		return fetchResult{Error: "python3 not found"}
	}

	scriptPath := "scripts/yad2_price_scraper.py"
	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, python, scriptPath,
		"--json",
		strconv.Itoa(subModelID),
		strconv.Itoa(year),
	)

	out, err := cmd.Output()
	if err != nil {
		logger.Debug("python scraper failed", "sub_model_id", subModelID, "year", year, "error", err)
		return fetchResult{Error: fmt.Sprintf("python scraper: %v", err)}
	}

	var result struct {
		BasePrice *int   `json:"base_price"`
		Title     string `json:"title"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return fetchResult{Error: fmt.Sprintf("parse scraper output: %v", err)}
	}

	if result.BasePrice != nil && *result.BasePrice > 0 {
		return fetchResult{
			BasePrice: *result.BasePrice,
			Title:     result.Title,
		}
	}

	errMsg := "no price from scraper"
	if result.Error != "" {
		errMsg = result.Error
	}
	return fetchResult{Error: errMsg}
}
