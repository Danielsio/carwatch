package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/pricelist"
	"github.com/dsionov/carwatch/internal/scoring"
	"github.com/dsionov/carwatch/internal/storage"
)

// ListingPipeline encapsulates the shared processing steps for converting
// raw listings into scored, enriched ListingRecords. Both the scheduler
// and the API refresh handler use this pipeline so that users always get
// fitness scores, deal scores, suspicious-listing detection, and base
// price enrichment regardless of how the listing was fetched.
type ListingPipeline struct {
	listings     storage.ListingStore // for prefill (LookupEnrichmentData); may be nil
	priceListSvc *pricelist.Service   // for base price enrichment; may be nil
	logger       *slog.Logger
}

// NewListingPipeline creates a ListingPipeline. Both listings and priceListSvc
// may be nil; the pipeline gracefully skips steps that require them.
func NewListingPipeline(listings storage.ListingStore, priceListSvc *pricelist.Service, logger *slog.Logger) *ListingPipeline {
	return &ListingPipeline{
		listings:     listings,
		priceListSvc: priceListSvc,
		logger:       logger,
	}
}

// ProcessParams holds the per-call parameters for a pipeline invocation.
type ProcessParams struct {
	ChatID      int64
	SearchID    int64
	SearchName  string
	MarketCache *scoring.MarketCache // optional; nil skips deal scoring

	// Search filter bounds (used for fitness scoring).
	PriceMax    int
	MaxKm       int
	MaxHand     int
	YearMin     int
	YearMax     int
	EngineMinCC int
}

// ProcessResult holds the output of a pipeline invocation.
type ProcessResult struct {
	Records  []storage.ListingRecord
	Listings []model.Listing // enriched listings (for notifications, etc.)
}

// Process converts raw listings into scored ListingRecords.
//
// Steps performed:
//  1. Prefill km/city/image from DB (if listings store is available)
//  2. Compute fitness score
//  3. Compute deal score (if market cache is available)
//  4. Detect suspicious listings (if market cache is available)
//  5. Enrich with base price (if pricelist service is available)
//  6. Build ListingRecords
func (p *ListingPipeline) Process(ctx context.Context, raw []model.RawListing, params ProcessParams) ProcessResult {
	log := p.logger.With("op", "pipeline", "search_id", params.SearchID, "chat_id", params.ChatID)

	p.prefillFromDB(ctx, raw, log)

	result := ProcessResult{
		Records:  make([]storage.ListingRecord, 0, len(raw)),
		Listings: make([]model.Listing, 0, len(raw)),
	}

	for i := range raw {
		l := raw[i]
		listing := p.scoreAndEnrich(ctx, l, params, log)
		rec := p.buildRecord(listing, params)
		result.Listings = append(result.Listings, listing)
		result.Records = append(result.Records, rec)
	}

	return result
}

// scoreAndEnrich computes fitness score, deal score, suspicious reasons,
// and base price for a single listing.
func (p *ListingPipeline) scoreAndEnrich(ctx context.Context, l model.RawListing, params ProcessParams, log *slog.Logger) model.Listing {
	listing := model.Listing{RawListing: l, SearchName: params.SearchName}

	// Look up market median before fitness scoring so the price dimension
	// can score against market value instead of just the budget cap.
	var medianPrice, medianKm, cohort int
	var marketOK bool
	if params.MarketCache != nil && l.Price > 0 {
		medianPrice, medianKm, cohort, marketOK = params.MarketCache.Lookup(l.Manufacturer, l.Model, l.Year)
	}

	fp := scoring.FitnessParams{
		Price:        l.Price,
		Km:           l.Km,
		Hand:         l.Hand,
		Year:         l.Year,
		EngineVolume: l.EngineVolume,
		PriceMax:     params.PriceMax,
		MaxKm:        params.MaxKm,
		MaxHand:      params.MaxHand,
		YearMin:      params.YearMin,
		YearMax:      params.YearMax,
		EngineMinCC:  params.EngineMinCC,
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

	// Enrich with base price.
	p.enrichWithBasePrice(ctx, &listing, log)

	return listing
}

// enrichWithBasePrice looks up the base (catalog) price for the listing.
func (p *ListingPipeline) enrichWithBasePrice(ctx context.Context, listing *model.Listing, log *slog.Logger) {
	if p.priceListSvc == nil {
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

	bp, ok := p.priceListSvc.Lookup(ctx, listing.SubModelID, listing.Year, listing.Token)
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

// prefillFromDB fills in km/city/image from the DB for listings that are
// missing those fields. This is a no-op if the listing store is nil.
func (p *ListingPipeline) prefillFromDB(ctx context.Context, listings []model.RawListing, log *slog.Logger) {
	if p.listings == nil {
		return
	}

	var tokens []string
	for i := range listings {
		if listings[i].Km <= 0 || listings[i].City == "" || listings[i].ImageURL == "" {
			tokens = append(tokens, listings[i].Token)
		}
	}
	if len(tokens) == 0 {
		return
	}

	data, err := p.listings.LookupEnrichmentData(ctx, tokens)
	if err != nil {
		log.Error("pipeline prefill from DB failed", "error", err)
		return
	}
	if len(data) == 0 {
		return
	}

	filled := 0
	for i := range listings {
		rec, ok := data[listings[i].Token]
		if !ok {
			continue
		}
		changed := false
		if listings[i].Km <= 0 && rec.Km > 0 {
			listings[i].Km = rec.Km
			changed = true
		}
		if listings[i].City == "" && rec.City != "" {
			listings[i].City = rec.City
			changed = true
		}
		if listings[i].ImageURL == "" && rec.ImageURL != "" {
			listings[i].ImageURL = rec.ImageURL
			changed = true
		}
		if changed {
			filled++
		}
	}
	if filled > 0 {
		log.Info("pipeline prefilled from DB", "filled", filled, "looked_up", len(tokens))
	}
}

// buildRecord converts an enriched model.Listing into a storage.ListingRecord.
func (p *ListingPipeline) buildRecord(listing model.Listing, params ProcessParams) storage.ListingRecord {
	rec := storage.ListingRecord{
		Token:        listing.Token,
		ChatID:       params.ChatID,
		SearchID:     params.SearchID,
		SearchName:   params.SearchName,
		Manufacturer: listing.Manufacturer,
		Model:        listing.Model,
		SubModel:     listing.SubModel,
		SubModelID:   listing.SubModelID,
		Year:         listing.Year,
		Price:        listing.Price,
		Km:           listing.Km,
		Hand:         listing.Hand,
		City:         listing.City,
		PageLink:     listing.PageLink,
		ImageURL:     listing.ImageURL,
		EngineVolume: listing.EngineVolume,
		HorsePower:   listing.HorsePower,
		EngineType:   listing.EngineType,
		GearBox:      listing.GearBox,
		Description:  listing.Description,
		IsCommercial: listing.Commercial,
		FitnessScore: &listing.FitnessScore,
		BasePrice:    listing.BasePrice,
		FirstSeenAt:  time.Now(),
	}
	if listing.DealScore != nil {
		rec.MedianPrice = &listing.DealScore.MedianPrice
		rec.CohortSize = &listing.DealScore.CohortSize
		rec.DealScore = &listing.DealScore.Score
	}
	return rec
}

// ProcessParamsFromSearch builds ProcessParams from a storage.Search and an
// optional market cache. This is a convenience for callers that have a
// storage.Search at hand (both the scheduler and the API).
func ProcessParamsFromSearch(search storage.Search, marketCache *scoring.MarketCache) ProcessParams {
	return ProcessParams{
		ChatID:      search.ChatID,
		SearchID:    search.ID,
		SearchName:  search.Name,
		MarketCache: marketCache,
		PriceMax:    search.PriceMax,
		MaxKm:       search.MaxKm,
		MaxHand:     search.MaxHand,
		YearMin:     search.YearMin,
		YearMax:     search.YearMax,
		EngineMinCC: search.EngineMinCC,
	}
}
