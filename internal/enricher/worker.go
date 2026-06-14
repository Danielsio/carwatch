package enricher

import (
	"context"
	"errors"
	"fmt"
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
	// Resolve car identity for structured logging. LookupEnrichmentData
	// only matches rows that already have some enrichment data; fall back
	// to LookupListingIdentity for brand-new unenriched listings.
	existing, err := w.listings.LookupEnrichmentData(ctx, []string{req.Token})
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to check enrichment status",
			"token", req.Token, "error", err.Error())
		return err
	}

	carAttrs := w.carAttrs(ctx, req, existing)

	if rec, ok := existing[req.Token]; ok && rec.Km > 0 && rec.City != "" && rec.ImageURL != "" {
		w.logger.DebugContext(ctx, "listing already enriched, skipping", carAttrs...)
		if telemetry.EnrichSkipped != nil {
			telemetry.EnrichSkipped.Add(ctx, 1)
		}
		return nil
	}

	if !w.limiter.Wait(ctx) {
		return ctx.Err()
	}

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
				append(carAttrs, "cooldown", w.limiter.InCooldown(), "current_delay", w.limiter.CurrentDelay())...)
			return fetchErr
		}

		if errors.Is(fetchErr, fetcher.ErrItemGone) {
			// The listing was removed at the source; mark permanent so the
			// consumer dead-letters immediately instead of retrying.
			w.logger.InfoContext(ctx, "listing no longer exists at source, will not retry",
				append(carAttrs, "error", fetchErr.Error())...)
			return fmt.Errorf("%w: %w", broker.ErrPermanent, fetchErr)
		}

		w.logger.WarnContext(ctx, "failed to fetch listing detail page",
			append(carAttrs, "error", fetchErr.Error())...)
		return fetchErr
	}

	w.limiter.RecordSuccess()

	rec := storage.ListingRecord{
		Token:    req.Token,
		Km:       details.Km,
		City:     details.City,
		ImageURL: details.ImageURL,
	}
	if err := w.listings.BackfillListings(ctx, []storage.ListingRecord{rec}); err != nil {
		w.logger.ErrorContext(ctx, "failed to backfill enriched data to database",
			append(carAttrs, "error", err.Error())...)
		return err
	}

	// Count toward the enrich_attempts cap when km is genuinely unavailable
	// on the source page, so backfill stops re-queuing this token.
	if details.Km <= 0 {
		if incErr := w.listings.IncrementEnrichAttempt(ctx, req.Token); incErr != nil {
			w.logger.ErrorContext(ctx, "failed to increment enrich attempt for km-unavailable listing",
				"token", req.Token, "error", incErr.Error())
		}
	}

	if telemetry.EnrichSuccesses != nil {
		telemetry.EnrichSuccesses.Add(ctx, 1)
	}

	w.logger.InfoContext(ctx, "enriched listing",
		append(carAttrs, "km", details.Km, "city", details.City, "has_image", details.ImageURL != "")...)

	return nil
}

// carAttrs builds a reusable slog attribute slice with car-identifying fields.
func (w *Worker) carAttrs(ctx context.Context, req broker.EnrichRequest, existing map[string]storage.EnrichmentRecord) []any {
	attrs := []any{"token", req.Token, "source", req.Source, "priority", req.Priority}
	if len(req.SearchIDs) > 0 {
		attrs = append(attrs, "search_ids", req.SearchIDs)
	}

	if rec, ok := existing[req.Token]; ok {
		return append(attrs,
			"manufacturer", rec.Manufacturer, "model", rec.Model,
			"year", rec.Year, "price", rec.Price, "search_name", rec.SearchName)
	}

	id, err := w.listings.LookupListingIdentity(ctx, req.Token)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			w.logger.WarnContext(ctx, "failed to lookup listing identity", "token", req.Token, "error", err.Error())
		}
		return attrs
	}
	return append(attrs,
		"manufacturer", id.Manufacturer, "model", id.Model,
		"year", id.Year, "price", id.Price, "search_name", id.SearchName)
}
