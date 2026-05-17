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

func (s *Scheduler) processSearchListings(ctx context.Context, search storage.Search, filtered []model.RawListing, marketCache *scoring.MarketCache, lang locale.Lang, log *slog.Logger) searchResult {
	var out searchResult
	hidden := s.loadHiddenTokens(ctx, search.ChatID)
	for _, l := range filterHiddenListings(filtered, hidden) {
		if !storage.RawListingMatchesSellerFilter(l.Commercial, search.SellerFilter) {
			continue
		}
		isNew, ok := s.deduplicateListings(ctx, l.Token, search.ChatID, search.ID, log)
		if !ok {
			continue
		}
		if s.tryPriceDropListing(ctx, search, l, lang, marketCache, &out, log) {
			continue
		}
		if !isNew {
			continue
		}
		listing := s.scoreAndRecordListings(search, l, marketCache)
		s.enrichWithBasePrice(ctx, &listing, log)
		buildNotifications(search, listing, &out)
	}
	return out
}

func (s *Scheduler) scoreAndRecordListings(search storage.Search, l model.RawListing, marketCache *scoring.MarketCache) model.Listing {
	listing := model.Listing{RawListing: l, SearchName: search.Name}

	// Look up market median before fitness scoring so the price dimension
	// can score against market value instead of just the budget cap.
	var medianPrice, medianKm, cohort int
	var marketOK bool
	if marketCache != nil && l.Price > 0 {
		medianPrice, medianKm, cohort, marketOK = marketCache.Lookup(l.Manufacturer, l.Model, l.Year)
	}

	fp := scoring.FitnessParams{
		Price:        l.Price,
		Km:           l.Km,
		Hand:         l.Hand,
		Year:         l.Year,
		EngineVolume: l.EngineVolume,
		PriceMax:     search.PriceMax,
		MaxKm:        search.MaxKm,
		MaxHand:      search.MaxHand,
		YearMin:      search.YearMin,
		YearMax:      search.YearMax,
		EngineMinCC:  search.EngineMinCC,
	}
	if marketOK {
		fp.MedianPrice = medianPrice
	}

	detailed := scoring.FitnessScoreDetailed(fp)
	listing.FitnessScore = detailed.Total
	listing.FitnessBreakdown = make([]model.FitnessDim, len(detailed.Dims))
	for i, d := range detailed.Dims {
		listing.FitnessBreakdown[i] = model.FitnessDim{
			Name: d.Name, Score: d.Score, Weight: d.Weight,
		}
	}
	if marketOK {
		listing.DealScore = &model.ScoreInfo{
			Score:       scoring.ScoreWithKm(l.Price, l.Km, medianPrice, medianKm),
			MedianPrice: medianPrice,
			MedianKm:    medianKm,
			CohortSize:  cohort,
		}
		listing.SuspiciousReasons = scoring.DetectSuspicious(l, medianPrice)
	}
	return listing
}

func buildNotifications(search storage.Search, listing model.Listing, out *searchResult) {
	out.newListings = append(out.newListings, listing)
	rec := storage.ListingRecord{
		Token: listing.Token, ChatID: search.ChatID, SearchID: search.ID, SearchName: search.Name,
		Manufacturer: listing.Manufacturer, Model: listing.Model, SubModel: listing.SubModel,
		SubModelID: listing.SubModelID,
		Year:       listing.Year, Price: listing.Price, Km: listing.Km, Hand: listing.Hand,
		City: listing.City, PageLink: listing.PageLink, ImageURL: listing.ImageURL,
		EngineVolume: listing.EngineVolume, HorsePower: listing.HorsePower,
		EngineType: listing.EngineType, GearBox: listing.GearBox, Description: listing.Description,
		IsCommercial: listing.Commercial,
		FitnessScore: &listing.FitnessScore, BasePrice: listing.BasePrice, FirstSeenAt: time.Now(),
	}
	if listing.DealScore != nil {
		rec.MedianPrice = &listing.DealScore.MedianPrice
		rec.CohortSize = &listing.DealScore.CohortSize
		rec.DealScore = &listing.DealScore.Score
	}
	out.listingRecords = append(out.listingRecords, rec)
}

func (s *Scheduler) enrichWithBasePrice(ctx context.Context, listing *model.Listing, log *slog.Logger) {
	if s.priceListSvc == nil {
		log.Debug("enrichWithBasePrice: skipped, pricelist service is nil",
			"token", listing.Token)
		return
	}
	if listing.SubModelID <= 0 {
		log.Debug("enrichWithBasePrice: skipped, no sub_model_id",
			"token", listing.Token, "sub_model_id", listing.SubModelID,
			"sub_model", listing.SubModel, "model", listing.Model)
		return
	}
	if listing.Year <= 0 {
		log.Debug("enrichWithBasePrice: skipped, no year",
			"token", listing.Token, "year", listing.Year)
		return
	}

	bp, ok := s.priceListSvc.Lookup(ctx, listing.SubModelID, listing.Year, listing.Token)
	if !ok {
		log.Warn("enrichWithBasePrice: lookup failed",
			"token", listing.Token, "sub_model_id", listing.SubModelID,
			"year", listing.Year)
		return
	}
	if bp <= 0 {
		log.Warn("enrichWithBasePrice: zero/negative price",
			"token", listing.Token, "sub_model_id", listing.SubModelID,
			"year", listing.Year, "base_price", bp)
		return
	}

	listing.BasePrice = &bp
	log.Info("enrichWithBasePrice: set base_price",
		"token", listing.Token, "sub_model_id", listing.SubModelID,
		"year", listing.Year, "base_price", bp)
}

func (s *Scheduler) deliverResults(ctx context.Context, search storage.Search, lang locale.Lang, sr searchResult, log *slog.Logger) bool {
	delivery := s.deliveryFor(ctx, search.ChatID, lang)
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
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
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
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
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

func filterHiddenListings(filtered []model.RawListing, hidden map[string]bool) []model.RawListing {
	if len(hidden) == 0 {
		return filtered
	}
	out := make([]model.RawListing, 0, len(filtered))
	for _, l := range filtered {
		if !hidden[l.Token] {
			out = append(out, l)
		}
	}
	return out
}
