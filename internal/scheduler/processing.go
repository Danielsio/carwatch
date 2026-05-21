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
				log.Warn("user blocked bot, deactivating")
				if s.stores.Users != nil {
					if err := s.stores.Users.SetUserActive(ctx, search.ChatID, false); err != nil {
						log.Error("set user inactive after block (price drop)", "error", err)
					}
				}
				return false
			}
			log.Error("price drop delivery failed", "error", err)
		} else {
			sent = true
		}
	}

	if len(sr.newListings) == 0 {
		return sent
	}

	s.observer.RecordListingsFound(len(sr.newListings))

	log.Info("notifying user of new listings", "count", len(sr.newListings))

	if err := delivery.DeliverBatch(ctx, search.ChatID, sr.newListings); err != nil {
		if errors.Is(err, notifier.ErrRecipientBlocked) {
			log.Warn("user blocked bot, deactivating")
			if s.stores.Users != nil {
				if err := s.stores.Users.SetUserActive(ctx, search.ChatID, false); err != nil {
					log.Error("set user inactive after block (batch)", "error", err)
				}
			}
		} else {
			log.Error("batch delivery failed", "count", len(sr.newListings), "error", err)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, l := range sr.newListings {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, l.Token, search.ChatID); relErr != nil {
				log.Error("release claim after delivery failure",
					"token", l.Token, "error", relErr)
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
		log.Error("record price failed", "token", l.Token, "error", err)
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
	log.Info("price drop detected",
		"token", l.Token,
		"old_price", oldPrice,
		"new_price", l.Price,
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
			log.Error("save price-drop listing failed",
				"token", l.Token,
				"error", err,
			)
		}
	}
	return true
}

func (s *Scheduler) persistListings(ctx context.Context, records []storage.ListingRecord, log *slog.Logger) error {
	if err := s.stores.Listings.SaveListings(ctx, records); err != nil {
		log.Error("batch save listings failed", "batch_size", len(records), "error", err)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, rec := range records {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, rec.Token, rec.ChatID); relErr != nil {
				log.Error("release claim after batch save failure",
					"token", rec.Token, "error", relErr)
			}
		}
		return err
	}
	return nil
}

func (s *Scheduler) deduplicateListings(ctx context.Context, token string, chatID, searchID int64, log *slog.Logger) (isNew bool, ok bool) {
	isNew, err := s.stores.Dedup.ClaimNew(ctx, token, chatID, searchID)
	if err != nil {
		log.Error("claim failed", "token", token, "error", err)
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
			log.Error("revert price after persist failure",
				"token", token, "error", err)
		}
	}
}

func (s *Scheduler) loadHiddenTokens(ctx context.Context, chatID int64) map[string]bool {
	if s.stores.Hidden == nil {
		return nil
	}
	tokens, err := s.stores.Hidden.ListHiddenTokens(ctx, chatID)
	if err != nil {
		s.logger.Error("load hidden tokens failed", "chat_id", chatID, "error", err)
		return nil
	}
	return tokens
}
