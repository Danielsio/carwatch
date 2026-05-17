package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/scoring"
)

type instantSearchRequest struct {
	Source       string `json:"source"`
	Manufacturer int    `json:"manufacturer"`
	Model        int    `json:"model"`
	YearMin      int    `json:"year_min"`
	YearMax      int    `json:"year_max"`
	PriceMin     int    `json:"price_min"`
	PriceMax     int    `json:"price_max"`
	MaxKm        int    `json:"max_km"`
	MaxHand      int    `json:"max_hand"`
	EngineMinCC  int    `json:"engine_min_cc"`
	GearBox      string `json:"gear_box,omitempty"`
	PriceOnly    bool   `json:"price_only,omitempty"`
	PhotoOnly    bool   `json:"photo_only,omitempty"`
}

type instantSearchResponse struct {
	Items []listingResponse `json:"items"`
	Total int               `json:"total"`
}

const instantSearchMaxResults = 30

func (s *Server) instantSearch(w http.ResponseWriter, r *http.Request) {
	if s.fetchers == nil {
		writeError(w, http.StatusServiceUnavailable, "search not available")
		return
	}

	var req instantSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Manufacturer is required for instant search to avoid unbounded result
	// sets â without it the fetcher would need to crawl all categories which
	// is slow and expensive (and the results would be too broad to be useful).
	if req.Manufacturer <= 0 {
		writeError(w, http.StatusBadRequest, "×××¨ ××¦×¨× ××× ×××¤×© / Please select a manufacturer to search")
		return
	}

	if req.Source == "" {
		req.Source = "yad2"
	}
	if !isValidSource(req.Source) {
		writeError(w, http.StatusBadRequest, "invalid source")
		return
	}

	if msg := validateSearchRanges(req.YearMin, req.YearMax, req.PriceMin, req.PriceMax, req.MaxKm, req.MaxHand, req.EngineMinCC); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	log := s.handlerLogger(r, "op", "instant_search")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	sources := strings.Split(req.Source, ",")
	var allRaw []model.RawListing
	for _, src := range sources {
		src = strings.TrimSpace(src)
		f, ok := s.fetchers.Get(src)
		if !ok {
			continue
		}
		params := model.SourceParams{
			Manufacturer: req.Manufacturer,
			Model:        req.Model,
			YearMin:      req.YearMin,
			YearMax:      req.YearMax,
			PriceMin:     req.PriceMin,
			PriceMax:     req.PriceMax,
			MaxKm:        req.MaxKm,
			MaxHand:      req.MaxHand,
			EngineMinCC:  req.EngineMinCC,
		}

		var raw []model.RawListing
		var fetchErr error
		for attempt := 0; attempt < 3; attempt++ {
			raw, fetchErr = f.Fetch(ctx, params)
			if fetchErr == nil {
				break
			}
			log.Warn("fetch attempt failed", "source", src,
				"attempt", fmt.Sprintf("%d/3", attempt+1), "error", fetchErr)
			if attempt < 2 {
				delay := time.Duration(1<<attempt) * time.Second
				select {
				case <-ctx.Done():
					break
				case <-time.After(delay):
				}
			}
		}
		if fetchErr != nil {
			continue
		}
		allRaw = append(allRaw, raw...)
	}

	criteria := model.FilterCriteria{
		ModelID:     req.Model,
		YearMin:     req.YearMin,
		YearMax:     req.YearMax,
		PriceMin:    req.PriceMin,
		PriceMax:    req.PriceMax,
		EngineMinCC: float64(req.EngineMinCC),
		MaxKm:       req.MaxKm,
		MaxHand:     req.MaxHand,
		GearBox:     req.GearBox,
		PriceOnly:   req.PriceOnly,
		PhotoOnly:   req.PhotoOnly,
	}
	filtered := filter.Apply(criteria, allRaw)

	// Build a market cache from the fetched listings themselves — the
	// same cohort that the guest just searched for. This enables deal
	// scoring and market comparison without needing persistent data.
	marketData := make([]scoring.ListingData, 0, len(filtered))
	for _, l := range filtered {
		if l.Price > 0 {
			marketData = append(marketData, scoring.ListingData{
				Manufacturer: l.Manufacturer,
				Model:        l.Model,
				Year:         l.Year,
				Price:        l.Price,
				Km:           l.Km,
			})
		}
	}
	marketCache := scoring.NewMarketCache(marketData)

	type scoredListing struct {
		listing    model.RawListing
		fitness    float64
		dealScore  *int
		median     *int
		medianKm   *int
		cohort     *int
		basePrice  *int
		suspicious []string
	}
	scored := make([]scoredListing, 0, len(filtered))
	for _, l := range filtered {
		fp := scoring.FitnessParams{
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			Year:         l.Year,
			EngineVolume: l.EngineVolume,
			PriceMax:     req.PriceMax,
			MaxKm:        req.MaxKm,
			MaxHand:      req.MaxHand,
			YearMin:      req.YearMin,
			YearMax:      req.YearMax,
			EngineMinCC:  req.EngineMinCC,
		}

		var sl scoredListing
		sl.listing = l

		if median, medKm, cohortSize, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
			fp.MedianPrice = median
			ds := scoring.ScoreWithKm(l.Price, l.Km, median, medKm)
			sl.dealScore = &ds
			sl.median = &median
			sl.medianKm = &medKm
			sl.cohort = &cohortSize
			sl.suspicious = scoring.DetectSuspicious(l, median)
		}

		result := scoring.FitnessScoreDetailed(fp)
		sl.fitness = result.Total

		scored = append(scored, sl)
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].fitness > scored[j].fitness
	})

	if len(scored) > instantSearchMaxResults {
		scored = scored[:instantSearchMaxResults]
	}

	// Enrich only the top results with pricelist base price to avoid
	// unnecessary external lookups for listings that won't be returned.
	if s.priceListSvc != nil {
		for i := range scored {
			l := scored[i].listing
			if l.SubModelID > 0 && l.Year > 0 {
				if bp, ok := s.priceListSvc.Lookup(ctx, l.SubModelID, l.Year, l.Token); ok && bp > 0 {
					scored[i].basePrice = &bp
				}
			}
		}
	}

	items := make([]listingResponse, 0, len(scored))
	for _, sl := range scored {
		l := sl.listing
		fs := sl.fitness
		resp := listingResponse{
			Token:             l.Token,
			Manufacturer:      l.Manufacturer,
			Model:             l.Model,
			SubModel:          l.SubModel,
			Year:              l.Year,
			Price:             l.Price,
			Km:                l.Km,
			Hand:              l.Hand,
			City:              l.City,
			PageLink:          l.PageLink,
			ImageURL:          l.ImageURL,
			EngineVolume:      l.EngineVolume,
			HorsePower:        l.HorsePower,
			EngineType:        l.EngineType,
			GearBox:           l.GearBox,
			Description:       l.Description,
			FitnessScore:      &fs,
			MedianPrice:       sl.median,
			CohortSize:        sl.cohort,
			DealScore:         sl.dealScore,
			BasePrice:         sl.basePrice,
			SuspiciousReasons: sl.suspicious,
			FirstSeenAt:       l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			IsCommercial:      l.Commercial,
		}
		items = append(items, resp)
	}

	log.Info("instant search complete",
		"fetched", len(allRaw), "filtered", len(filtered), "returned", len(items))

	writeJSON(w, http.StatusOK, instantSearchResponse{
		Items: items,
		Total: len(items),
	})
}
