package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/scoring"
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
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, l := range sr.newListings {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, l.Token, search.ChatID); relErr != nil {
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

func (s *Scheduler) tryPriceDropListing(ctx context.Context, search storage.Search, l model.RawListing, lang locale.Lang, marketCache *scoring.MarketCache, out *searchResult, log *slog.Logger) bool {
	if s.stores.Prices == nil || l.Price <= 0 {
		return false
	}
	oldPrice, changed, err := s.stores.Prices.RecordPrice(ctx, l.Token, l.Price)
	if err != nil {
		log.ErrorContext(ctx, "failed to record listing price in price tracker",
			"token", l.Token,
			"error", err.Error(),
			"impact", "price drop detection will not work for this listing this cycle",
			"action_taken", "skipping price drop check, listing will be processed as normal")
		return false
	}
	// Track tokens where RecordPrice inserted a row so that we can revert
	// them if downstream persistence fails (prevents spurious price-drop
	// notifications on the next scheduler cycle).
	// A row is inserted when: (a) first observation (changed=false, oldPrice=0)
	// or (b) price actually changed (changed=true).
	if changed || oldPrice == 0 {
		out.recordedTokens = append(out.recordedTokens, l.Token)
	}
	if !changed || l.Price >= oldPrice {
		return false
	}
	log.InfoContext(ctx, "detected price drop for matched listing, preparing to notify user",
		"token", l.Token,
		"old_price", oldPrice,
		"new_price", l.Price,
		"price_change", l.Price-oldPrice,
		"manufacturer", l.Manufacturer,
		"model", l.Model,
		"year", l.Year,
	)
	listing := model.Listing{RawListing: l, SearchName: search.Name}
	fp := scoring.FitnessParams{
		Price: l.Price, Km: l.Km, Hand: l.Hand, Year: l.Year,
		EngineVolume: l.EngineVolume, PriceMax: search.PriceMax,
		MaxKm: search.MaxKm, MaxHand: search.MaxHand,
		YearMin: search.YearMin, YearMax: search.YearMax,
		EngineMinCC: search.EngineMinCC,
	}
	if marketCache != nil {
		if median, _, _, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
			fp.MedianPrice = median
		}
	}
	listing.FitnessScore = scoring.FitnessScore(fp)
	out.priceDropMessages = append(out.priceDropMessages, notifier.FormatPriceDrop(listing, oldPrice, lang))
	if s.stores.Listings != nil {
		rec := storage.ListingRecord{
			Token: l.Token, ChatID: search.ChatID, SearchID: search.ID, SearchName: search.Name,
			Manufacturer: l.Manufacturer, Model: l.Model, SubModel: l.SubModel,
			SubModelID: l.SubModelID,
			Year:       l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
			City: l.City, PageLink: l.PageLink, ImageURL: l.ImageURL,
			EngineVolume: l.EngineVolume, HorsePower: l.HorsePower,
			EngineType: l.EngineType, GearBox: l.GearBox, Description: l.Description,
			IsCommercial: l.Commercial,
			FitnessScore: &listing.FitnessScore, FirstSeenAt: time.Now(),
		}
		if marketCache != nil {
			if median, medKm, cohort, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
				ds := scoring.ScoreWithKm(l.Price, l.Km, median, medKm)
				rec.MedianPrice = &median
				rec.CohortSize = &cohort
				rec.DealScore = &ds
			}
		}
		if err := s.stores.Listings.SaveListing(ctx, rec); err != nil {
			log.ErrorContext(ctx, "failed to persist price-drop listing to history",
				"token", l.Token,
				"error", err.Error(),
				"impact", "price drop record will be missing from listing history",
				"action_taken", "notification will still be sent, but history is incomplete")
		}
	}
	return true
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
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, rec := range records {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, rec.Token, rec.ChatID); relErr != nil {
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
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
