package yad2

import (
	"context"
	"log/slog"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

const (
	defaultEnrichDelay = 500 * time.Millisecond
	defaultMaxEnrich   = 5
)

// EnricherConfig controls the enrichment behavior.
type EnricherConfig struct {
	// Delay between individual item fetches to avoid rate-limiting.
	Delay time.Duration
	// MaxPerCycle limits how many listings are enriched per poll cycle.
	MaxPerCycle int
}

// Enricher fills in missing km data by fetching individual listing pages.
type Enricher struct {
	fetcher *Yad2Fetcher
	logger  *slog.Logger
	cfg     EnricherConfig
}

// NewEnricher creates an Enricher backed by the given Yad2Fetcher.
func NewEnricher(fetcher *Yad2Fetcher, logger *slog.Logger, cfg EnricherConfig) *Enricher {
	if cfg.Delay == 0 {
		cfg.Delay = defaultEnrichDelay
	}
	if cfg.MaxPerCycle <= 0 {
		cfg.MaxPerCycle = defaultMaxEnrich
	}
	return &Enricher{fetcher: fetcher, logger: logger, cfg: cfg}
}

// Enrich fills in Km for listings where it is zero.
// It modifies the slice in place and returns the number of successfully enriched listings.
func (e *Enricher) Enrich(ctx context.Context, listings []model.RawListing) int {
	enriched := 0
	for i := range listings {
		if listings[i].Km > 0 {
			continue
		}
		if enriched >= e.cfg.MaxPerCycle {
			e.logger.Info("km enrichment limit reached", "enriched", enriched, "remaining", countMissingKm(listings[i:]))
			break
		}

		if enriched > 0 {
			select {
			case <-ctx.Done():
				return enriched
			case <-time.After(e.cfg.Delay):
			}
		}

		details, err := e.fetcher.FetchItem(ctx, listings[i].Token)
		if err != nil {
			e.logger.Warn("km enrichment failed",
				"token", listings[i].Token,
				"error", err,
			)
			continue
		}

		if details.Km > 0 {
			listings[i].Km = details.Km
			enriched++
			e.logger.Debug("enriched km",
				"token", listings[i].Token,
				"km", details.Km,
			)
		}
	}
	return enriched
}

func countMissingKm(listings []model.RawListing) int {
	n := 0
	for _, l := range listings {
		if l.Km <= 0 {
			n++
		}
	}
	return n
}
