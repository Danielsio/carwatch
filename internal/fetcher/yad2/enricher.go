package yad2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

const (
	defaultEnrichDelay     = 2 * time.Second
	defaultMaxPerCycle     = 25
	maxConsecutiveFailures = 3
	// challengeBackoff is how long to wait after a bot challenge before retrying.
	challengeBackoff = 7 * time.Second
	// maxChallengeRetries is how many consecutive challenges we tolerate before aborting.
	maxChallengeRetries = 2
)

// EnricherConfig controls the enrichment behavior.
type EnricherConfig struct {
	// Delay between individual item fetches to avoid rate-limiting.
	// The actual delay is randomized: [Delay*0.75, Delay*1.75].
	Delay time.Duration
	// MaxPerCycle caps successful enrichments per Enrich call (not fetch attempts).
	// 0 (default) applies defaultMaxPerCycle (25); override in config as needed. Negative means no limit.
	MaxPerCycle int
	// ChallengeBackoff is how long to wait after a bot challenge before retrying.
	// 0 (default) uses 7 seconds.
	ChallengeBackoff time.Duration
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
	if cfg.MaxPerCycle == 0 {
		cfg.MaxPerCycle = defaultMaxPerCycle
	}
	if cfg.ChallengeBackoff == 0 {
		cfg.ChallengeBackoff = challengeBackoff
	}
	return &Enricher{fetcher: fetcher, logger: logger, cfg: cfg}
}

// jitteredDelay returns a randomized duration in [base*0.75, base*1.75].
func jitteredDelay(base time.Duration) time.Duration {
	lo := float64(base) * 0.75
	hi := float64(base) * 1.75
	return time.Duration(lo + rand.Float64()*(hi-lo))
}

// Enrich fills in missing Km, ImageURL, and City for listings.
// It shuffles the candidates so different listings are prioritized each cycle.
// It modifies the slice in place and returns the number of successfully enriched listings.
func (e *Enricher) Enrich(ctx context.Context, listings []model.RawListing) int {
	// Build shuffled indices of listings needing enrichment.
	var candidates []int
	for i := range listings {
		if listings[i].Km <= 0 || listings[i].ImageURL == "" || listings[i].City == "" {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return 0
	}
	rand.Shuffle(len(candidates), func(a, b int) {
		candidates[a], candidates[b] = candidates[b], candidates[a]
	})

	enriched := 0
	attempts := 0
	consecutiveFailures := 0
	challengeRetries := 0

	e.logger.InfoContext(ctx, "enrichment pass started",
		"candidates", len(candidates), "delay", e.cfg.Delay, "max_per_cycle", e.cfg.MaxPerCycle)
	passStart := time.Now()

	for _, i := range candidates {
		if e.cfg.MaxPerCycle > 0 && enriched >= e.cfg.MaxPerCycle {
			e.logger.DebugContext(ctx, "reached per-cycle enrichment limit, deferring remaining listings",
				"enriched", enriched,
				"attempts", attempts,
				"remaining", len(candidates)-attempts,
			)
			break
		}

		select {
		case <-ctx.Done():
			return enriched
		default:
		}

		if attempts > 0 {
			delay := jitteredDelay(e.cfg.Delay)
			select {
			case <-ctx.Done():
				return enriched
			case <-time.After(delay):
			}
		}

		attempts++
		fetchStart := time.Now()
		details, err := e.fetcher.FetchItem(ctx, listings[i].Token)
		if err != nil {
			fetchElapsed := time.Since(fetchStart)
			if errors.Is(err, fetcher.ErrChallenge) {
				challengeRetries++
				if challengeRetries >= maxChallengeRetries {
					e.logger.WarnContext(ctx, "halting enrichment pass, source is rate-limiting with bot challenges",
						"token", listings[i].Token,
						"car", listings[i].Manufacturer+" "+listings[i].Model,
						"challenges", challengeRetries,
						"fetch_ms", fetchElapsed.Milliseconds(),
						"impact", fmt.Sprintf("%d remaining listings will not be enriched this cycle", len(candidates)-attempts),
						"action_taken", "aborting enrichment, will retry unenriched listings next cycle",
						"enriched_so_far", enriched,
						"attempts", attempts,
					)
					return enriched
				}
				e.logger.WarnContext(ctx, "bot challenge detected during enrichment, backing off before retry",
					"token", listings[i].Token,
					"car", listings[i].Manufacturer+" "+listings[i].Model,
					"challenge_count", challengeRetries,
					"fetch_ms", fetchElapsed.Milliseconds(),
					"backoff", e.cfg.ChallengeBackoff,
				)
				select {
				case <-ctx.Done():
					return enriched
				case <-time.After(e.cfg.ChallengeBackoff):
				}
				continue
			}
			consecutiveFailures++
			e.logger.WarnContext(ctx, "failed to fetch detailed listing data for mileage enrichment",
				"token", listings[i].Token,
				"car", listings[i].Manufacturer+" "+listings[i].Model,
				"error", err.Error(),
				"fetch_ms", fetchElapsed.Milliseconds(),
				"impact", "listing will appear without km/city data until next cycle",
				"action_taken", "continuing with remaining candidates",
				"consecutive_failures", consecutiveFailures,
			)
			if consecutiveFailures >= maxConsecutiveFailures {
				e.logger.WarnContext(ctx, "halting enrichment pass after consecutive fetch failures",
					"token", listings[i].Token,
					"car", listings[i].Manufacturer+" "+listings[i].Model,
					"consecutive_failures", consecutiveFailures,
					"impact", fmt.Sprintf("%d remaining listings will not be enriched this cycle", len(candidates)-attempts),
					"action_taken", "aborting enrichment, unenriched listings will be retried next cycle",
					"enriched_so_far", enriched,
				)
				return enriched
			}
			continue
		}

		fetchElapsed := time.Since(fetchStart)
		consecutiveFailures = 0
		challengeRetries = 0
		changed := false
		if listings[i].Km <= 0 && details.Km > 0 {
			listings[i].Km = details.Km
			changed = true
		}
		if listings[i].ImageURL == "" && details.ImageURL != "" {
			listings[i].ImageURL = details.ImageURL
			changed = true
		}
		if listings[i].City == "" && details.City != "" {
			listings[i].City = details.City
			changed = true
		}
		if listings[i].Area == "" && details.Area != "" {
			listings[i].Area = details.Area
			changed = true
		}
		if listings[i].OriginalOwnership == "" && details.OriginalOwnership != "" {
			listings[i].OriginalOwnership = details.OriginalOwnership
			changed = true
		}
		if listings[i].BodyType == "" && details.BodyType != "" {
			listings[i].BodyType = details.BodyType
			changed = true
		}
		if changed {
			enriched++
			e.logger.InfoContext(ctx, "enriched listing",
				"token", listings[i].Token,
				"car", listings[i].Manufacturer+" "+listings[i].Model,
				"km", details.Km,
				"city", details.City,
				"has_image", details.ImageURL != "",
				"fetch_ms", fetchElapsed.Milliseconds(),
				"progress", fmt.Sprintf("%d/%d", enriched, len(candidates)),
			)
		}
	}

	e.logger.InfoContext(ctx, "enrichment pass completed",
		"enriched", enriched, "attempts", attempts, "candidates", len(candidates),
		"elapsed", time.Since(passStart).Round(time.Millisecond))
	return enriched
}
