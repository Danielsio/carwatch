package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Scheduler) deliverResults(ctx context.Context, search storage.Search, lang locale.Lang, sr searchResult, log *slog.Logger) bool {
	delivery := s.deliveryFor(ctx, search.ChatID, lang, search.ID, search.Name, log)
	sent := false

	for _, msg := range sr.priceDropMessages {
		if err := delivery.DeliverRaw(ctx, search.ChatID, msg); err != nil {
			if errors.Is(err, notifier.ErrRecipientBlocked) {
				log.WarnContext(ctx, "user has blocked the bot, deactivating user account",
					"impact", "user will stop receiving all notifications",
					"action_taken", "setting user inactive to prevent further delivery attempts")
				if s.stores.Users != nil {
					if err := s.stores.Users.SetUserActive(ctx, search.ChatID, false); err != nil {
						log.ErrorContext(ctx, "failed to deactivate user after bot block",
							"error", err.Error(),
							"impact", "user remains active and delivery will fail again next cycle",
							"action_taken", "no further action, will retry deactivation next cycle")
					}
				}
				return false
			}
			log.ErrorContext(ctx, "failed to deliver price drop notification to user",
				"error", err.Error(),
				"impact", "user will not be notified about this price drop",
				"action_taken", "continuing with remaining notifications")
		} else {
			sent = true
		}
	}

	if len(sr.newListings) == 0 {
		return sent
	}

	s.observer.RecordListingsFound(len(sr.newListings))

	log.InfoContext(ctx, "delivering batch of newly matched listings to user",
		"count", len(sr.newListings),
		"search_name", search.Name,
	)

	if err := delivery.DeliverBatch(ctx, search.ChatID, sr.newListings); err != nil {
		if errors.Is(err, errMalformedMessage) {
			log.WarnContext(ctx, "batch message is malformed, keeping dedup claims to prevent infinite retry",
				"count", len(sr.newListings), "search_name", search.Name)
			return false
		}
		if errors.Is(err, notifier.ErrRecipientBlocked) {
			log.WarnContext(ctx, "user has blocked the bot during batch delivery, deactivating user account",
				"impact", "user will stop receiving all notifications",
				"action_taken", "setting user inactive, releasing dedup claims for retry")
			if s.stores.Users != nil {
				if err := s.stores.Users.SetUserActive(ctx, search.ChatID, false); err != nil {
					log.ErrorContext(ctx, "failed to deactivate user after bot block during batch delivery",
						"error", err.Error(),
						"impact", "user remains active and delivery will fail again next cycle",
						"action_taken", "no further action, will retry deactivation next cycle")
				}
			}
		} else {
			log.ErrorContext(ctx, "failed to deliver listing batch to user, releasing dedup claims for retry",
				"count", len(sr.newListings),
				"error", err.Error(),
				"impact", "user will not see these listings until next successful delivery",
				"action_taken", "releasing dedup claims so listings are retried next cycle")
		}
		// Survive the cancellation that may have caused the failure, but keep
		// the request's trace/log context (WithoutCancel) instead of a bare
		// Background.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		for _, l := range sr.newListings {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, l.Token, search.ChatID, search.ID); relErr != nil {
				log.ErrorContext(ctx, "failed to release dedup claim after delivery failure",
					"token", l.Token,
					"error", relErr.Error(),
					"impact", "this listing may not be re-delivered on subsequent cycles",
					"action_taken", "manual intervention may be needed to clear stuck claim")
			}
		}
	} else {
		s.observer.RecordNotificationSent()
		sent = true
	}
	return sent
}

func (s *Scheduler) persistListings(ctx context.Context, records []storage.ListingRecord, log *slog.Logger) error {
	saveStart := time.Now()
	if err := s.stores.Listings.SaveListings(ctx, records); err != nil {
		log.ErrorContext(ctx, "failed to persist matched listings to database, releasing all dedup claims for retry",
			"batch_size", len(records),
			"duration_ms", time.Since(saveStart).Milliseconds(),
			"error", err.Error(),
			"impact", "listings will not appear in user history until next successful cycle",
			"action_taken", "releasing dedup claims so listings are retried next cycle")
		// Survive cancellation but preserve trace/log context (WithoutCancel).
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		for _, rec := range records {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, rec.Token, rec.ChatID, rec.SearchID); relErr != nil {
				log.ErrorContext(ctx, "failed to release dedup claim after batch save failure, listing may be permanently lost",
					"token", rec.Token,
					"error", relErr.Error(),
					"impact", "this listing will not be re-delivered on subsequent cycles",
					"action_taken", "manual intervention may be needed to clear stuck claim")
			}
		}
		return err
	}
	return nil
}

func (s *Scheduler) deduplicateListings(ctx context.Context, token string, chatID, searchID int64, log *slog.Logger) (isNew bool, ok bool) {
	isNew, err := s.stores.Dedup.ClaimNew(ctx, token, chatID, searchID)
	if err != nil {
		log.ErrorContext(ctx, "failed to claim listing for deduplication, listing will be skipped this cycle",
			"token", token,
			"error", err.Error(),
			"impact", "listing may be re-delivered on next successful cycle if claim is not persisted",
			"action_taken", "skipping this listing to avoid duplicate delivery")
		return false, false
	}
	return isNew, true
}

// revertPriceRecords undoes RecordPrice calls for the given tokens.
// Called when persistListings fails to avoid stale price records that
// would cause spurious price-drop notifications on the next cycle.
func (s *Scheduler) revertPriceRecords(ctx context.Context, tokens []string, log *slog.Logger) {
	if s.stores.Prices == nil || len(tokens) == 0 {
		return
	}
	// Survive cancellation but preserve trace/log context (WithoutCancel).
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	for _, token := range tokens {
		if err := s.stores.Prices.RevertPrice(cleanupCtx, token); err != nil {
			log.ErrorContext(ctx, "failed to revert price record after persist failure, may cause spurious price-drop notification",
				"token", token,
				"error", err.Error(),
				"impact", "stale price record may trigger a false price-drop notification on next cycle",
				"action_taken", "continuing with remaining reverts")
		}
	}
}

func (s *Scheduler) loadHiddenTokens(ctx context.Context, chatID int64) map[string]bool {
	if s.stores.Hidden == nil {
		return nil
	}
	tokens, err := s.stores.Hidden.ListHiddenTokens(ctx, chatID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load hidden tokens for user, hidden listings may be re-delivered",
			"chat_id", chatID,
			"error", err.Error(),
			"impact", "previously hidden listings may appear in notifications",
			"action_taken", "continuing without hidden token filter")
		return nil
	}
	return tokens
}
