package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
)

type catalogEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameHe string `json:"name_he"`
}

type catalogOutput struct {
	Manufacturers []catalogEntry            `json:"manufacturers"`
	Models        map[string][]catalogEntry `json:"models"`
}

func main() {
	var (
		output   string
		dryRun   bool
		delay    time.Duration
		mfrIDs   string
		maxPages int
	)
	flag.StringVar(&output, "output", "internal/catalog/catalog_data.json", "output JSON file path")
	flag.BoolVar(&dryRun, "dry-run", false, "print to stdout instead of writing file")
	flag.DurationVar(&delay, "delay", 3*time.Second, "delay between requests to avoid anti-bot")
	flag.StringVar(&mfrIDs, "manufacturers", "", "comma-separated manufacturer IDs to scrape (empty = use existing catalog as base)")
	flag.IntVar(&maxPages, "max-pages", 1, "max pages to fetch per manufacturer")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	userAgents := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	}

	fetcher, err := yad2.NewFetcher(userAgents, "", logger)
	if err != nil {
		log.Fatalf("failed to create fetcher: %v", err)
	}
	defer fetcher.Close()

	pageFetcher := &catalog.HTTPPageFetcher{
		GetPage: fetcher.FetchRawPage,
	}

	result := &catalogOutput{
		Models: make(map[string][]catalogEntry),
	}

	existing := loadExistingCatalog(output)

	ctx := context.Background()

	logger.Info("fetching main cars page for manufacturer/model discovery")
	catResult, err := catalog.FetchCatalogFromYad2(ctx, pageFetcher)
	if err != nil {
		logger.Warn("failed to fetch catalog from main page, using existing as base", "error", err)
		if existing != nil {
			result = existing
		}
	} else {
		for id, entry := range catResult.Manufacturers {
			result.Manufacturers = append(result.Manufacturers, catalogEntry{
				ID:     id,
				Name:   entry.Name,
				NameHe: entry.NameHe,
			})
		}
		for mfrID, models := range catResult.Models {
			key := fmt.Sprintf("%d", mfrID)
			for id, entry := range models {
				result.Models[key] = append(result.Models[key], catalogEntry{
					ID:     id,
					Name:   entry.Name,
					NameHe: entry.NameHe,
				})
			}
		}
	}

	if existing != nil {
		mergeExisting(result, existing)
	}

	if mfrIDs != "" {
		ids := parseIDs(mfrIDs)
		for i, id := range ids {
			if i > 0 {
				logger.Info("sleeping between requests", "delay", delay)
				time.Sleep(delay)
			}
			logger.Info("fetching manufacturer page", "id", id, "page", 1)
			scrapeManufacturer(ctx, pageFetcher, result, id, maxPages, delay, logger)
		}
	}

	sort.Slice(result.Manufacturers, func(i, j int) bool {
		return strings.ToLower(result.Manufacturers[i].Name) < strings.ToLower(result.Manufacturers[j].Name)
	})
	for key, models := range result.Models {
		sort.Slice(models, func(i, j int) bool {
			return strings.ToLower(models[i].Name) < strings.ToLower(models[j].Name)
		})
		result.Models[key] = models
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}

	if dryRun {
		fmt.Println(string(data))
		return
	}

	if err := os.WriteFile(output, data, 0644); err != nil {
		log.Fatalf("failed to write file %s: %v", output, err)
	}
	logger.Info("catalog written",
		"path", output,
		"manufacturers", len(result.Manufacturers),
		"model_groups", len(result.Models),
	)
}

func scrapeManufacturer(ctx context.Context, fetcher catalog.Yad2PageFetcher, result *catalogOutput, mfrID int, maxPages int, delay time.Duration, logger *slog.Logger) {
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("https://www.yad2.co.il/vehicles/cars?manufacturer=%d&page=%d", mfrID, page)
		body, err := fetcher.FetchPage(ctx, url)
		if err != nil {
			logger.Warn("failed to fetch manufacturer page", "id", mfrID, "page", page, "error", err)
			return
		}

		catResult, err := catalog.ParseCatalogFromHTML(body)
		if err != nil {
			logger.Warn("failed to parse manufacturer page", "id", mfrID, "page", page, "error", err)
			return
		}

		key := fmt.Sprintf("%d", mfrID)
		existingModels := make(map[int]bool)
		for _, m := range result.Models[key] {
			existingModels[m.ID] = true
		}

		newCount := 0
		for id, entry := range catResult.Models[mfrID] {
			if !existingModels[id] {
				result.Models[key] = append(result.Models[key], catalogEntry{
					ID:     id,
					Name:   entry.Name,
					NameHe: entry.NameHe,
				})
				existingModels[id] = true
				newCount++
			}
		}

		for id, entry := range catResult.Manufacturers {
			found := false
			for _, m := range result.Manufacturers {
				if m.ID == id {
					found = true
					break
				}
			}
			if !found {
				result.Manufacturers = append(result.Manufacturers, catalogEntry{
					ID:     id,
					Name:   entry.Name,
					NameHe: entry.NameHe,
				})
			}
		}

		logger.Info("scraped manufacturer page", "id", mfrID, "page", page, "new_models", newCount)

		if page < maxPages {
			time.Sleep(delay)
		}
	}
}

func loadExistingCatalog(path string) *catalogOutput {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out catalogOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	if out.Models == nil {
		out.Models = make(map[string][]catalogEntry)
	}
	return &out
}

func mergeExisting(result, existing *catalogOutput) {
	existingMfrs := make(map[int]bool)
	for _, m := range result.Manufacturers {
		existingMfrs[m.ID] = true
	}
	for _, m := range existing.Manufacturers {
		if !existingMfrs[m.ID] {
			result.Manufacturers = append(result.Manufacturers, m)
		}
	}

	for key, models := range existing.Models {
		existingModels := make(map[int]bool)
		for _, m := range result.Models[key] {
			existingModels[m.ID] = true
		}
		for _, m := range models {
			if !existingModels[m.ID] {
				result.Models[key] = append(result.Models[key], m)
			}
		}
	}
}

func parseIDs(s string) []int {
	parts := strings.Split(s, ",")
	var ids []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(p, "%d", &id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
