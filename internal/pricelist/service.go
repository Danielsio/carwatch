package pricelist

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

const (
	cacheTTL         = 7 * 24 * time.Hour
	maxFetchPerCycle = 20
	fetchCooldown    = 2 * time.Second
)

type Service struct {
	store  storage.PriceListStore
	client HTTPDoer
	logger *slog.Logger

	mu         sync.Mutex
	lastFetch  time.Time
	fetchCount int
}

func NewService(store storage.PriceListStore, client HTTPDoer, logger *slog.Logger) *Service {
	return &Service{
		store:  store,
		client: client,
		logger: logger,
	}
}

// ResetCycleCounter resets the per-cycle fetch counter. Call at the start of each scan cycle.
func (s *Service) ResetCycleCounter() {
	s.mu.Lock()
	s.fetchCount = 0
	s.mu.Unlock()
}

// Lookup returns the cached base price for a sub-model/year pair.
// If not cached or stale, it fetches from Yad2 (rate-limited).
func (s *Service) Lookup(ctx context.Context, subModelID, year int) (basePrice int, ok bool) {
	if subModelID <= 0 || year <= 0 {
		s.logger.Debug("pricelist.Lookup: invalid params",
			"sub_model_id", subModelID, "year", year)
		return 0, false
	}

	entry, err := s.store.GetPriceListEntry(ctx, subModelID, year)
	if err != nil {
		s.logger.Error("pricelist.Lookup: cache read failed",
			"sub_model_id", subModelID, "year", year, "error", err)
		return 0, false
	}

	if entry != nil && time.Since(entry.FetchedAt) < cacheTTL && entry.BasePrice > 0 {
		s.logger.Debug("pricelist.Lookup: cache hit",
			"sub_model_id", subModelID, "year", year,
			"base_price", entry.BasePrice, "age", time.Since(entry.FetchedAt).Round(time.Minute))
		return entry.BasePrice, true
	}

	s.mu.Lock()
	if s.fetchCount >= maxFetchPerCycle {
		s.mu.Unlock()
		s.logger.Warn("pricelist.Lookup: cycle fetch limit reached",
			"sub_model_id", subModelID, "year", year,
			"fetch_count", s.fetchCount, "max", maxFetchPerCycle)
		if entry != nil {
			return entry.BasePrice, true
		}
		return 0, false
	}
	elapsed := time.Since(s.lastFetch)
	if elapsed < fetchCooldown {
		s.mu.Unlock()
		s.logger.Debug("pricelist.Lookup: cooldown active",
			"sub_model_id", subModelID, "year", year,
			"remaining", (fetchCooldown - elapsed).Round(time.Millisecond))
		if entry != nil {
			return entry.BasePrice, true
		}
		return 0, false
	}
	s.fetchCount++
	s.lastFetch = time.Now()
	s.mu.Unlock()

	url := priceListURL(subModelID, year)
	s.logger.Info("pricelist.Lookup: fetching from Yad2",
		"sub_model_id", subModelID, "year", year, "url", url,
		"fetch_count", s.fetchCount)

	result := fetch(ctx, s.client, subModelID, year)
	if result.Error != "" {
		s.logger.Warn("pricelist.Lookup: fetch failed",
			"sub_model_id", subModelID, "year", year,
			"url", url, "error", result.Error)
	}
	if result.BasePrice <= 0 {
		s.logger.Warn("pricelist.Lookup: no price extracted",
			"sub_model_id", subModelID, "year", year,
			"url", url, "fetch_error", result.Error)
		if entry != nil {
			return entry.BasePrice, true
		}
		return 0, false
	}

	newEntry := storage.PriceListEntry{
		SubModelID: subModelID,
		Year:       year,
		BasePrice:  result.BasePrice,
		Title:      result.Title,
		FetchedAt:  time.Now(),
	}
	if err := s.store.SetPriceListEntry(ctx, newEntry); err != nil {
		s.logger.Error("pricelist.Lookup: cache write failed",
			"sub_model_id", subModelID, "year", year, "error", err)
	}

	s.logger.Info("pricelist.Lookup: fetched successfully",
		"sub_model_id", subModelID, "year", year,
		"base_price", result.BasePrice, "title", result.Title)

	return result.BasePrice, true
}

type fetchResult struct {
	BasePrice int
	Title     string
	Error     string
}

func priceListURL(subModelID, year int) string {
	return fmt.Sprintf("https://www.yad2.co.il/price-list/sub-model/%d/%d", subModelID, year)
}
