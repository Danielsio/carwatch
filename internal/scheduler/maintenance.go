package scheduler

import (
	"context"
	"time"

	"github.com/dsionov/carwatch/internal/broker"
)

func (s *Scheduler) pruneOldData(ctx context.Context) {
	if time.Since(s.lastPruneTime) <= pruneInterval {
		return
	}
	s.cfgMu.RLock()
	pruneAfter := s.cfg.Storage.PruneAfter
	s.cfgMu.RUnlock()
	if pruneAfter > 0 {
		pruned, err := s.stores.Dedup.Prune(ctx, pruneAfter)
		if err != nil {
			s.logger.Error("prune dedup failed", "retention", pruneAfter.String(), "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old dedup entries", "count", pruned, "retention", pruneAfter.String())
		}
	}
	if s.stores.Prices != nil {
		pruned, err := s.stores.Prices.PrunePrices(ctx, priceHistoryRetention)
		if err != nil {
			s.logger.Error("prune prices failed", "retention", priceHistoryRetention.String(), "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old price history", "count", pruned, "retention", priceHistoryRetention.String())
		}
	}
	if s.stores.Listings != nil {
		pruned, err := s.stores.Listings.PruneListings(ctx, listingHistoryRetention)
		if err != nil {
			s.logger.Error("prune listing history failed", "retention", listingHistoryRetention.String(), "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listing history", "count", pruned, "retention", listingHistoryRetention.String())
		}
	}
	s.lastPruneTime = time.Now()
}

func (s *Scheduler) backfillUnenrichedListings(ctx context.Context) {
	if s.enrichPublisher == nil || s.stores.Listings == nil {
		return
	}

	tokens, err := s.stores.Listings.ListUnenrichedTokens(ctx, 15)
	if err != nil {
		s.logger.Error("list unenriched tokens failed", "error", err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	s.logger.Info("backfill enrichment: publishing unenriched tokens to stream",
		"candidates", len(tokens),
	)

	published := 0
	for _, t := range tokens {
		req := broker.EnrichRequest{
			Token:      t,
			Priority:   3,
			Source:     "backfill",
			EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.enrichPublisher.PublishEnrich(ctx, req); err != nil {
			s.logger.Warn("failed to publish backfill enrichment request to stream",
				"token", t, "error", err)
		} else {
			published++
		}
	}
	if published > 0 {
		s.logger.Info("published backfill enrichment requests to stream",
			"published", published, "candidates", len(tokens))
	}
}

func (s *Scheduler) processExpiredPremium(ctx context.Context) {
	if s.stores.Users == nil {
		return
	}
	expired, err := s.stores.Users.ListExpiredPremium(ctx)
	if err != nil {
		s.logger.Error("list expired premium users failed", "error", err)
		return
	}
	if len(expired) == 0 {
		return
	}
	s.cfgMu.RLock()
	maxSearches := s.cfg.Telegram.MaxSearches
	s.cfgMu.RUnlock()
	for _, u := range expired {
		if err := s.stores.Users.SetUserTier(ctx, u.ChatID, "free", time.Time{}); err != nil {
			s.logger.Error("downgrade expired premium user failed",
				"chat_id", u.ChatID,
				"error", err,
			)
			continue
		}
		s.deactivateExcessSearches(ctx, u.ChatID, maxSearches)
		s.logger.Info("premium subscription expired, user downgraded to free",
			"chat_id", u.ChatID,
		)
	}
}
