package enricher

import (
	"context"
	"errors"
	"log/slog"

	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/telemetry"
)

// ItemDetails holds enrichment data fetched from an individual listing page.
type ItemDetails struct {
	Km       int
	ImageURL string
	City     string
	Area     string
}

// ItemFetcher fetches detailed data for a single listing by token.
type ItemFetcher interface {
	FetchItem(ctx context.Context, token string) (ItemDetails, error)
}

// Worker processes enrichment requests from a Redis stream by fetching
// individual listing pages and updating the database.
type Worker struct {
	fetcher  ItemFetcher
	listings storage.ListingStore
	limiter  *AdaptiveRateLimiter
	logger   *slog.Logger
}

// NewWorker creates an enrichment worker.
func NewWorker(f ItemFetcher, ls storage.ListingStore, rl *AdaptiveRateLimiter, logger *slog.Logger) *Worker {
	return &Worker{
		fetcher:  f,
		listings: ls,
		limiter:  rl,
		logger:   logger,
	}
}

// HandleRequest is the broker.EnrichFunc callback. It processes a single
// enrichment request: checks if already enriched, waits for rate limit,
// fetches the item page, and updates the database.
func (w *Worker) HandleRequest(ctx context.Context, req broker.EnrichRequest) error {
	// Check if already enriched (idempotent skip).
	// A listing is considered fully enriched only when all key fields exist.
	// If one field is missing (for example only image exists), keep trying.
	existing, err := w.listings.LookupEnrichmentData(ctx, []string{req.Token})
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to check enrichment status",
			"token", req.Token, "error", err.Error())
		return err
	}
	if rec, ok := existing[req.Token]; ok && rec.Km > 0 && rec.City != "" && rec.ImageURL != "" {
		hasKm := rec.Km > 0
		hasCity := rec.City != ""
		hasImage := rec.ImageURL != ""
		w.logger.DebugContext(ctx, "listing already enriched, skipping",
			"token", req.Token, "km", rec.Km, "city", rec.City,
			"has_km", hasKm, "has_city", hasCity, "has_image", hasImage,
			"skip_reason", "already_fully_enriched")
		if telemetry.EnrichSkipped != nil {
			telemetry.EnrichSkipped.Add(ctx, 1)
		}
		return nil
	}

	// Wait for rate limiter.
	if !w.limiter.Wait(ctx) {
		return ctx.Err()
	}

	// Fetch item detail page.
	details, fetchErr := w.fetcher.FetchItem(ctx, req.Token)
	if fetchErr != nil {
		if incErr := w.listings.IncrementEnrichAttempt(ctx, req.Token); incErr != nil {
			w.logger.ErrorContext(ctx, "failed to increment enrich attempt",
				"token", req.Token, "error", incErr.Error())
		}

		if errors.Is(fetchErr, fetcher.ErrChallenge) {
			w.limiter.RecordChallenge()
			if telemetry.EnrichChallenges != nil {
				telemetry.EnrichChallenges.Add(ctx, 1)
			}
			w.logger.WarnContext(ctx, "bot challenge during enrichment, backing off",
				"token", req.Token, "cooldown", w.limiter.InCooldown(),
				"current_delay", w.limiter.CurrentDelay())
			return fetchErr
		}

		w.logger.WarnContext(ctx, "failed to fetch listing detail page",
			"token", req.Token, "error", fetchErr.Error())
		return fetchErr
	}

	w.limiter.RecordSuccess()

	// Build a minimal listing record for backfill.
	rec := storage.ListingRecord{
		Token:    req.Token,
		Km:       details.Km,
		City:     details.City,
		ImageURL: details.ImageURL,
	}
	if err := w.listings.BackfillListings(ctx, []storage.ListingRecord{rec}); err != nil {
		w.logger.ErrorContext(ctx, "failed to backfill enriched data to database",
			"token", req.Token, "error", err.Error())
		return err
	}

	if telemetry.EnrichSuccesses != nil {
		telemetry.EnrichSuccesses.Add(ctx, 1)
	}

	w.logger.InfoContext(ctx, "enriched listing",
		"token", req.Token, "km", details.Km, "city", details.City,
		"has_image", details.ImageURL != "", "priority", req.Priority)

	return nil
}
