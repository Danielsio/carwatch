package scheduler

import (
	"context"
	"time"

	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/cwlog"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/percolator"
	"github.com/dsionov/carwatch/internal/scoring"
	"github.com/dsionov/carwatch/internal/storage"
)

type searchAccum struct {
	search storage.Search
	result searchResult
	lang   locale.Lang
}

type searchCycleCounter struct {
	matched    int
	kmFiltered int
}

type priceDropCandidate struct {
	rawIdx   int
	searchID int64
	oldPrice int
}

type priceResult struct {
	oldPrice int
	changed  bool
	err      bool
}

type matchState struct {
	raw             []model.RawListing
	feedTokensByKey map[string][]string
	prefilled       bool
	accums          map[int64]*searchAccum
	searchCounters  map[int64]*searchCycleCounter
	matchedIndices  map[int][]int64
	priceDropCandidates []priceDropCandidate
	pendingPrices       map[string]int
	persistedTokens     map[string]bool
	persistFailedTokens map[string]bool
}

func newMatchState() *matchState {
	return &matchState{
		accums:              make(map[int64]*searchAccum),
		searchCounters:      make(map[int64]*searchCycleCounter),
		matchedIndices:      make(map[int][]int64),
		pendingPrices:       make(map[string]int),
		persistedTokens:     make(map[string]bool),
		persistFailedTokens: make(map[string]bool),
	}
}

func (s *Scheduler) fetchRawListings(ctx context.Context, searches []storage.Search) *matchState {
	ms := newMatchState()
	ms.raw, ms.feedTokensByKey = s.fetchTargetedListings(ctx, searches, nil, s.targetedFetcher)
	return ms
}

func (s *Scheduler) ingestCatalog(ctx context.Context, raw []model.RawListing) {
	if s.catalogIngester == nil {
		return
	}
	for _, l := range raw {
		s.catalogIngester.Ingest(ctx, catalog.IngestEntry{
			ManufacturerID:     l.ManufacturerID,
			ManufacturerName:   l.Manufacturer,
			ManufacturerNameHe: l.ManufacturerNameHe,
			ModelID:            l.ModelID,
			ModelName:          l.Model,
			ModelNameHe:        l.ModelNameHe,
		})
	}
}

func (s *Scheduler) matchListingsToSearches(ctx context.Context, ms *matchState) {
	hiddenCache := make(map[int64]map[string]bool)
	priceCache := make(map[string]priceResult)

	for i := range ms.raw {
		matches := s.percolator.Match(ms.raw[i])
		if len(matches) == 0 {
			continue
		}
		if _, seen := ms.matchedIndices[i]; !seen {
			ids := make([]int64, 0, len(matches))
			for _, m := range matches {
				ids = append(ids, m.SearchID)
			}
			ms.matchedIndices[i] = ids
		}

		for _, m := range matches {
			acc, ok := ms.accums[m.SearchID]
			if !ok {
				lang := s.userLang(ctx, m.ChatID)
				acc = &searchAccum{search: m.Search, lang: lang}
				ms.accums[m.SearchID] = acc
			}

			ctr, ok := ms.searchCounters[m.SearchID]
			if !ok {
				ctr = &searchCycleCounter{}
				ms.searchCounters[m.SearchID] = ctr
			}
			ctr.matched++

			hidden, cached := hiddenCache[m.ChatID]
			if !cached {
				hidden = s.loadHiddenTokens(ctx, m.ChatID)
				hiddenCache[m.ChatID] = hidden
			}
			if len(hidden) > 0 && hidden[ms.raw[i].Token] {
				continue
			}

			matchLog := s.logger.With("search_id", m.SearchID, "chat_id", m.ChatID, "search_name", m.SearchName)
			isNew, ok := s.deduplicateListings(ctx, ms.raw[i].Token, m.ChatID, m.SearchID, matchLog)
			if !ok {
				continue
			}

			if s.stores.Prices != nil && ms.raw[i].Price > 0 {
				pr, cached := priceCache[ms.raw[i].Token]
				if !cached {
					prevPrice, found, err := s.stores.Prices.PeekPrice(ctx, ms.raw[i].Token)
					if err != nil {
						matchLog.ErrorContext(ctx, "failed to read last recorded listing price",
							"token", ms.raw[i].Token, "error", err.Error())
						pr = priceResult{err: true}
					} else {
						pr = priceResult{oldPrice: prevPrice, changed: found && ms.raw[i].Price != prevPrice}
						if !found || pr.changed {
							ms.pendingPrices[ms.raw[i].Token] = ms.raw[i].Price
						}
					}
					priceCache[ms.raw[i].Token] = pr
				}
				if !pr.err && pr.changed && ms.raw[i].Price < pr.oldPrice {
					ms.priceDropCandidates = append(ms.priceDropCandidates, priceDropCandidate{
						rawIdx: i, searchID: m.SearchID, oldPrice: pr.oldPrice,
					})
					continue
				}
			}

			if !isNew {
				continue
			}

			acc.result.newListings = append(acc.result.newListings, model.Listing{RawListing: ms.raw[i]})
		}
	}
}

func (s *Scheduler) publishEnrichRequests(ctx context.Context, ms *matchState) {
	if s.enrichPublisher == nil || len(ms.matchedIndices) == 0 {
		return
	}
	published := 0
	for idx, searchIDs := range ms.matchedIndices {
		l := ms.raw[idx]
		if l.Km <= 0 || l.City == "" || l.ImageURL == "" {
			req := broker.EnrichRequest{
				Token:      l.Token,
				Priority:   1,
				SearchIDs:  searchIDs,
				Source:     "scheduler",
				EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
			}
			if err := s.enrichPublisher.PublishEnrich(ctx, req); err != nil {
				s.logger.WarnContext(ctx, "failed to publish enrichment request to stream",
					"token", l.Token, "car", l.Manufacturer+" "+l.Model, "error", err)
			} else {
				published++
			}
		}
	}
	if published > 0 {
		s.logger.InfoContext(ctx, "published enrichment requests for matched listings",
			"published", published, "total_matched", len(ms.matchedIndices))
	}
}

func (s *Scheduler) processPriceDrops(ctx context.Context, ms *matchState, marketCache *scoring.MarketCache) {
	for _, pd := range ms.priceDropCandidates {
		acc := ms.accums[pd.searchID]
		if acc == nil {
			continue
		}
		l := ms.raw[pd.rawIdx]
		s.logger.InfoContext(ctx, "detected price drop for matched listing, preparing to notify user",
			"token", l.Token, "old_price", pd.oldPrice, "new_price", l.Price,
			"price_change", l.Price-pd.oldPrice,
			"manufacturer", l.Manufacturer, "model", l.Model, "year", l.Year,
		)
		listing := model.Listing{RawListing: l, SearchName: acc.search.Name}
		fp := scoring.FitnessParams{
			Price: l.Price, Km: l.Km, Hand: l.Hand, Year: l.Year,
			EngineVolume: l.EngineVolume, PriceMax: acc.search.PriceMax,
			MaxKm: acc.search.MaxKm, MaxHand: acc.search.MaxHand,
			YearMin: acc.search.YearMin, YearMax: acc.search.YearMax,
			EngineMinCC: acc.search.EngineMinCC,
		}
		if marketCache != nil {
			if median, medKm, _, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
				fp.MedianPrice = median
				fp.MedianKm = medKm
			}
		}
		listing.FitnessScore = scoring.FitnessScore(fp)
		acc.result.priceDropMessages = append(acc.result.priceDropMessages, notifier.FormatPriceDrop(listing, pd.oldPrice, acc.lang))
		if s.stores.Listings != nil {
			rec := storage.ListingRecord{
				Token: l.Token, ChatID: acc.search.ChatID, SearchID: acc.search.ID, SearchName: acc.search.Name,
				Manufacturer: l.Manufacturer, Model: l.Model, SubModel: l.SubModel,
				SubModelID: l.SubModelID,
				Year: l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
				City: l.City, PageLink: l.PageLink, ImageURL: l.ImageURL,
				EngineVolume: l.EngineVolume, HorsePower: l.HorsePower,
				EngineType: l.EngineType, GearBox: l.GearBox, Description: l.Description,
				IsCommercial: l.Commercial,
				FitnessScore: &listing.FitnessScore, FirstSeenAt: time.Now(),
			}
			if !l.CreatedAt.IsZero() {
				t := l.CreatedAt
				rec.PostedAt = &t
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
				s.logger.ErrorContext(ctx, "failed to persist price-drop listing to history",
					"token", l.Token, "error", err.Error())
			}
		}
	}
}

func (s *Scheduler) processAndDeliverPerSearch(ctx context.Context, ms *matchState, marketCache *scoring.MarketCache) int {
	rawByToken := make(map[string]int, len(ms.raw))
	for i := range ms.raw {
		rawByToken[ms.raw[i].Token] = i
	}

	newListings := 0
	notificationsSent := 0

	for _, acc := range ms.accums {
		searchCtx := cwlog.WithSearch(ctx, acc.search.ID, acc.search.ChatID)

		if len(acc.result.newListings) > 0 {
			rawForPipeline := make([]model.RawListing, len(acc.result.newListings))
			for i, l := range acc.result.newListings {
				if idx, ok := rawByToken[l.Token]; ok {
					rawForPipeline[i] = ms.raw[idx]
				} else {
					rawForPipeline[i] = l.RawListing
				}
			}

			if len(rawForPipeline) == 0 {
				acc.result.newListings = nil
			} else {
				params := ProcessParamsFromSearch(acc.search, marketCache)
				params.SkipPrefill = ms.prefilled
				pr := s.pipeline.Process(searchCtx, rawForPipeline, params)
				acc.result.newListings = pr.Listings
				acc.result.listingRecords = pr.Records
			}

			if acc.search.MaxKm > 0 && len(acc.result.newListings) > 0 {
				var deliverable []model.Listing
				for _, l := range acc.result.newListings {
					if l.Km <= 0 || l.Km > acc.search.MaxKm {
						reason := "km_exceeded"
						if l.Km <= 0 {
							reason = "km_pending_enrichment"
						}
						if relErr := s.stores.Dedup.ReleaseClaim(ctx, l.Token, acc.search.ChatID, acc.search.ID); relErr != nil {
							s.logger.ErrorContext(searchCtx,
								"failed to release dedup claim for km-filtered listing",
								"token", l.Token, "reason", reason, "error", relErr.Error())
						}
						if ctr := ms.searchCounters[acc.search.ID]; ctr != nil {
							ctr.kmFiltered++
						}
						continue
					}
					deliverable = append(deliverable, l)
				}
				if len(deliverable) < len(acc.result.newListings) {
					s.logger.InfoContext(searchCtx,
						"filtered listings by km limit after enrichment",
						"search_name", acc.search.Name,
						"chat_id", acc.search.ChatID,
						"before", len(acc.result.newListings),
						"after", len(deliverable),
						"max_km", acc.search.MaxKm,
					)
					acc.result.newListings = deliverable
				}
			}
		}

		searchLog := s.logger.With("chat_id", acc.search.ChatID, "search_name", acc.search.Name)
		persistOK := true
		if s.stores.Listings != nil && len(acc.result.listingRecords) > 0 {
			if err := s.persistListings(searchCtx, acc.result.listingRecords, searchLog); err != nil {
				persistOK = false
				for _, rec := range acc.result.listingRecords {
					ms.persistFailedTokens[rec.Token] = true
				}
			} else {
				s.invalidateMarketCache()
				for _, rec := range acc.result.listingRecords {
					ms.persistedTokens[rec.Token] = true
				}
			}
		}
		if !persistOK {
			acc.result.newListings = nil
			acc.result.listingRecords = nil
		}

		newListings += len(acc.result.newListings)

		graceSec := s.cfg.Polling.EnrichGraceSeconds
		if graceSec <= 0 {
			graceSec = 60
		}
		if graceSec > 0 && len(acc.result.newListings) > 0 {
			var ready []model.Listing
			deferred := 0
			for _, l := range acc.result.newListings {
				unenriched := l.Km <= 0 || l.City == "" || l.ImageURL == ""
				justSeen := !l.CreatedAt.IsZero() && time.Since(l.CreatedAt) < time.Duration(graceSec)*time.Second
				if unenriched && justSeen {
					deferred++
					continue
				}
				ready = append(ready, l)
			}
			if deferred > 0 {
				searchLog.InfoContext(searchCtx, "deferred unenriched listings for enrich grace window",
					"deferred", deferred, "ready", len(ready), "grace_seconds", graceSec)
			}
			acc.result.newListings = ready
		}

		if len(acc.result.newListings) > 0 || len(acc.result.priceDropMessages) > 0 {
			searchLog.InfoContext(searchCtx, "search matched listings, preparing delivery",
				"new", len(acc.result.newListings),
				"price_drops", len(acc.result.priceDropMessages),
			)
		}
		delivered := s.deliverResults(searchCtx, acc.search, acc.lang, acc.result, searchLog)
		if delivered {
			notificationsSent++
		}
	}

	return notificationsSent
}

func (s *Scheduler) dropStaleListings(ctx context.Context, searches []storage.Search, ms *matchState) map[int64]int64 {
	droppedBySearch := make(map[int64]int64)
	if s.stores.Listings == nil || len(ms.feedTokensByKey) == 0 {
		return droppedBySearch
	}
	for _, search := range searches {
		params := model.SourceParamsFromSearch(&search)
		key := fetcher.CacheKeyFor(params)
		feedTokens, ok := ms.feedTokensByKey[key]
		if !ok {
			continue
		}
		dropped, err := s.stores.Listings.DeleteStaleListings(ctx, search.ChatID, search.ID, feedTokens)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to drop stale listings",
				"search_id", search.ID, "chat_id", search.ChatID, "error", err)
			continue
		}
		if dropped > 0 {
			droppedBySearch[search.ID] = dropped
			s.logger.InfoContext(ctx, "dropped stale listings",
				"search_id", search.ID, "search_name", search.Name,
				"chat_id", search.ChatID, "dropped", dropped)
		}
	}
	return droppedBySearch
}

func (s *Scheduler) writeCycleStats(ctx context.Context, searches []storage.Search, ms *matchState, raw []model.RawListing, droppedBySearch map[int64]int64) {
	if s.stores.SearchCycleStats == nil || len(searches) == 0 {
		return
	}
	rejections := s.percolator.CountRejections(raw)
	now := time.Now()

	cycleStats := make([]storage.SearchCycleStats, 0, len(searches))
	for _, search := range searches {
		acc := ms.accums[search.ID]
		ctr := ms.searchCounters[search.ID]
		matched := 0
		kmFiltered := 0
		if ctr != nil {
			matched = ctr.matched
			kmFiltered = ctr.kmFiltered
		}

		rej := rejections[search.ID]

		var newListingCount, priceDropCount int
		var scoreMin, scoreMax, scoreAvg *float64
		var priceMin, priceMax *int
		if acc != nil && len(acc.result.newListings) > 0 {
			newListingCount = len(acc.result.newListings)
			priceDropCount = len(acc.result.priceDropMessages)
			mn, mx, sum := acc.result.newListings[0].FitnessScore, acc.result.newListings[0].FitnessScore, 0.0
			pMin, pMax := acc.result.newListings[0].Price, acc.result.newListings[0].Price
			for _, l := range acc.result.newListings {
				if l.FitnessScore < mn {
					mn = l.FitnessScore
				}
				if l.FitnessScore > mx {
					mx = l.FitnessScore
				}
				sum += l.FitnessScore
				if l.Price > 0 {
					if pMin == 0 || l.Price < pMin {
						pMin = l.Price
					}
					if l.Price > pMax {
						pMax = l.Price
					}
				}
			}
			avg := sum / float64(len(acc.result.newListings))
			scoreMin = &mn
			scoreMax = &mx
			scoreAvg = &avg
			if pMin > 0 {
				priceMin = &pMin
				priceMax = &pMax
			}
		} else if acc != nil {
			priceDropCount = len(acc.result.priceDropMessages)
		}

		cycleStats = append(cycleStats, storage.SearchCycleStats{
			SearchID:    search.ID,
			ChatID:      search.ChatID,
			SearchName:  search.Name,
			CycleAt:     now,
			FeedSize:    matched + rej[percolator.RejectYearOut] + rej[percolator.RejectPriceOut] + rej[percolator.RejectKmOver] + rej[percolator.RejectHandOver] + rej[percolator.RejectEngineCC] + rej[percolator.RejectSeller] + rej[percolator.RejectOtherFilter],
			Matched:     matched,
			NewListings: newListingCount,
			KmFiltered:  kmFiltered,
			Delivered:   newListingCount,
			PriceDrops:  priceDropCount,
			WrongModel:  rej[percolator.RejectWrongModel],
			YearOut:     rej[percolator.RejectYearOut],
			PriceOut:    rej[percolator.RejectPriceOut],
			KmOver:      rej[percolator.RejectKmOver],
			HandOver:    rej[percolator.RejectHandOver],
			EngineCC:    rej[percolator.RejectEngineCC],
			Seller:      rej[percolator.RejectSeller],
			OtherFilter: rej[percolator.RejectOtherFilter],
			Dropped:     int(droppedBySearch[search.ID]),
			ScoreMin:    scoreMin,
			ScoreMax:    scoreMax,
			ScoreAvg:    scoreAvg,
			PriceMin:    priceMin,
			PriceMax:    priceMax,
		})
	}
	if err := s.stores.SearchCycleStats.UpsertSearchCycleStats(ctx, cycleStats); err != nil {
		s.logger.Error("failed to write per-search cycle stats", "error", err)
	}
}
