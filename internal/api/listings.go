package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type listingResponse struct {
	Token        string   `json:"token"`
	SearchName   string   `json:"search_name,omitempty"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	SubModel     string   `json:"sub_model,omitempty"`
	Year         int      `json:"year"`
	Price        int      `json:"price"`
	Km           int      `json:"km"`
	Hand         int      `json:"hand"`
	City         string   `json:"city"`
	PageLink     string   `json:"page_link"`
	ImageURL     string   `json:"image_url,omitempty"`
	EngineVolume float64  `json:"engine_volume,omitempty"`
	HorsePower   int      `json:"horse_power,omitempty"`
	EngineType   string   `json:"engine_type,omitempty"`
	GearBox      string   `json:"gear_box,omitempty"`
	Description  string   `json:"description,omitempty"`
	FitnessScore *float64 `json:"fitness_score,omitempty"`
	MedianPrice  *int     `json:"median_price,omitempty"`
	CohortSize   *int     `json:"cohort_size,omitempty"`
	DealScore    *int     `json:"deal_score,omitempty"`
	BasePrice    *int     `json:"base_price,omitempty"`
	FirstSeenAt  string   `json:"first_seen_at"`
	Saved        bool     `json:"saved,omitempty"`
	// Seen: user dismissed this listing from the new-items feed (notifications).
	Seen bool `json:"seen,omitempty"`
	// IsCommercial: omitted when unknown; false = private seller; true = dealer/commercial.
	IsCommercial *bool `json:"is_commercial,omitempty"`
}

type listingsPageResponse struct {
	Items  []listingResponse `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

type refreshResponse struct {
	Items   []listingResponse `json:"items"`
	Total   int64             `json:"total"`
	Removed int64             `json:"removed"`
}

func (s *Server) refreshListings(w http.ResponseWriter, r *http.Request) {
	if s.fetchers == nil {
		writeError(w, http.StatusServiceUnavailable, "refresh not available")
		return
	}

	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	cooldownKey := fmt.Sprintf("%d:%d", chatID, id)
	if ts, loaded := s.refreshMu.Load(cooldownKey); loaded {
		if last, ok := ts.(time.Time); ok {
			remaining := 60*time.Second - time.Since(last)
			if remaining > 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(remaining.Seconds())+1))
				writeError(w, http.StatusTooManyRequests,
					fmt.Sprintf("please wait %d seconds before refreshing again", int(remaining.Seconds())+1))
				return
			}
		}
	}

	sr, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		s.logger.Error("refresh: get search", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if sr == nil {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	source := sr.Source
	if source == "" {
		source = "yad2"
	}
	sources := strings.Split(source, ",")
	var allRaw []model.RawListing
	for _, src := range sources {
		src = strings.TrimSpace(src)
		f, ok := s.fetchers.Get(src)
		if !ok {
			continue
		}
		params := model.SourceParams{
			Manufacturer: sr.Manufacturer,
			Model:        sr.Model,
			YearMin:      sr.YearMin,
			YearMax:      sr.YearMax,
			PriceMax:     sr.PriceMax,
			MaxKm:        sr.MaxKm,
			MaxHand:      sr.MaxHand,
			EngineMinCC:  sr.EngineMinCC,
		}
		raw, fetchErr := f.Fetch(r.Context(), params)
		if fetchErr != nil {
			s.logger.Warn("refresh: fetch failed", "source", src, "error", fetchErr)
			continue
		}
		allRaw = append(allRaw, raw...)
	}

	s.refreshMu.Store(cooldownKey, time.Now())

	criteria := buildFilterCriteriaFromSearch(sr)
	filtered := filter.Apply(criteria, allRaw)

	freshTokens := make([]string, len(filtered))
	for i, l := range filtered {
		freshTokens[i] = l.Token
	}

	removed, err := s.listings.DeleteStaleListings(r.Context(), chatID, sr.ID, freshTokens)
	if err != nil {
		s.logger.Error("refresh: delete stale", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to clean stale listings")
		return
	}

	records := make([]storage.ListingRecord, 0, len(filtered))
	for _, l := range filtered {
		rec := storage.ListingRecord{
			Token:        l.Token,
			ChatID:       chatID,
			SearchID:     sr.ID,
			SearchName:   sr.Name,
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			SubModel:     l.SubModel,
			SubModelID:   l.SubModelID,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			City:         l.City,
			PageLink:     l.PageLink,
			ImageURL:     l.ImageURL,
			EngineVolume: l.EngineVolume,
			HorsePower:   l.HorsePower,
			EngineType:   l.EngineType,
			GearBox:      l.GearBox,
			Description:  l.Description,
			IsCommercial: l.Commercial,
			FirstSeenAt:  time.Now(),
		}
		if s.priceListSvc != nil && l.SubModelID > 0 && l.Year > 0 {
			if bp, ok := s.priceListSvc.Lookup(r.Context(), l.SubModelID, l.Year); ok && bp > 0 {
				rec.BasePrice = &bp
				s.logger.Debug("refresh: enriched with base_price",
					"token", l.Token, "sub_model_id", l.SubModelID,
					"year", l.Year, "base_price", bp)
			}
		}
		records = append(records, rec)
	}
	if len(records) > 0 {
		if err := s.listings.SaveListings(r.Context(), records); err != nil {
			s.logger.Error("refresh: save listings", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save listings")
			return
		}
	}

	lf := listingFilterFromSearch(sr)
	listings, err := s.listings.ListSearchListings(r.Context(), chatID, sr.ID, lf, 100, 0, "newest")
	if err != nil {
		s.logger.Error("refresh: list listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list listings")
		return
	}
	total, err := s.listings.CountSearchListings(r.Context(), chatID, sr.ID, lf)
	if err != nil {
		s.logger.Error("refresh: count listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count listings")
		return
	}

	savedMap := s.savedLookupForRecords(r.Context(), chatID, listings)
	seenMap := s.seenLookupForRecords(r.Context(), chatID, listings)

	writeJSON(w, http.StatusOK, refreshResponse{
		Items:   toListingResponses(listings, savedMap, seenMap),
		Total:   total,
		Removed: removed,
	})
}

func buildFilterCriteriaFromSearch(sr *storage.Search) model.FilterCriteria {
	criteria := model.FilterCriteria{
		ModelID:     sr.Model,
		YearMin:     sr.YearMin,
		YearMax:     sr.YearMax,
		PriceMax:    sr.PriceMax,
		EngineMinCC: float64(sr.EngineMinCC),
		MaxKm:       sr.MaxKm,
		MaxHand:     sr.MaxHand,
		PriceOnly:   sr.PriceOnly,
		PhotoOnly:   sr.PhotoOnly,
	}

	if sr.Keywords != "" {
		for _, kw := range strings.Split(sr.Keywords, ",") {
			if kw = strings.TrimSpace(kw); kw != "" {
				criteria.Keywords = append(criteria.Keywords, kw)
			}
		}
	}
	if sr.ExcludeKeys != "" {
		for _, ex := range strings.Split(sr.ExcludeKeys, ",") {
			if ex = strings.TrimSpace(ex); ex != "" {
				criteria.ExcludeKeys = append(criteria.ExcludeKeys, ex)
			}
		}
	}
	return criteria
}

func (s *Server) getListing(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	l, err := s.listings.GetListing(r.Context(), chatID, token)
	if err != nil {
		s.logger.Error("get listing", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get listing")
		return
	}
	if l == nil {
		writeError(w, http.StatusNotFound, "listing not found")
		return
	}

	savedFlag := false
	if s.saved != nil {
		var err error
		savedFlag, err = s.saved.IsSaved(r.Context(), chatID, token)
		if err != nil {
			s.logger.Error("is saved", "error", err)
		}
	}

	seenFlag := false
	seenMap := s.seenLookupForRecords(r.Context(), chatID, []storage.ListingRecord{{Token: token}})
	if seenMap != nil && seenMap[token] {
		seenFlag = true
	}

	writeJSON(w, http.StatusOK, listingResponse{
		Token:        l.Token,
		SearchName:   l.SearchName,
		Manufacturer: l.Manufacturer,
		Model:        l.Model,
		SubModel:     l.SubModel,
		Year:         l.Year,
		Price:        l.Price,
		Km:           l.Km,
		Hand:         l.Hand,
		City:         l.City,
		PageLink:     l.PageLink,
		ImageURL:     l.ImageURL,
		EngineVolume: l.EngineVolume,
		HorsePower:   l.HorsePower,
		EngineType:   l.EngineType,
		GearBox:      l.GearBox,
		Description:  l.Description,
		FitnessScore: l.FitnessScore,
		MedianPrice:  l.MedianPrice,
		CohortSize:   l.CohortSize,
		DealScore:    l.DealScore,
		BasePrice:    l.BasePrice,
		FirstSeenAt:  l.FirstSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
		Saved:        savedFlag,
		Seen:         seenFlag,
		IsCommercial: l.IsCommercial,
	})
}

func listingFilterFromSearch(sr *storage.Search) storage.ListingFilter {
	f := storage.ListingFilter{
		PriceMax:  sr.PriceMax,
		YearMin:   sr.YearMin,
		YearMax:   sr.YearMax,
		MaxKm:     sr.MaxKm,
		MaxHand:   sr.MaxHand,
		PriceOnly: sr.PriceOnly,
		PhotoOnly: sr.PhotoOnly,
	}
	switch storage.NormalizeSellerFilter(sr.SellerFilter) {
	case storage.SellerFilterPrivate:
		v := false
		f.Commercial = &v
	case storage.SellerFilterCommercial:
		v := true
		f.Commercial = &v
	}
	return f
}

func (s *Server) listListings(w http.ResponseWriter, r *http.Request) {
	chatID, okChat := requireChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	sr, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		s.logger.Error("get search for listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if sr == nil {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	limit, ok := parseIntParam(w, r, "limit", 20)
	if !ok {
		return
	}
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	offset, ok := parseIntParam(w, r, "offset", 0)
	if !ok {
		return
	}
	sort := parseSortParam(r)
	f := listingFilterFromSearch(sr)

	listings, err := s.listings.ListSearchListings(r.Context(), chatID, sr.ID, f, limit, offset, sort)
	if err != nil {
		s.logger.Error("list search listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list listings")
		return
	}

	total, err := s.listings.CountSearchListings(r.Context(), chatID, sr.ID, f)
	if err != nil {
		s.logger.Error("count search listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count listings")
		return
	}

	savedMap := s.savedLookupForRecords(r.Context(), chatID, listings)
	seenMap := s.seenLookupForRecords(r.Context(), chatID, listings)

	writeJSON(w, http.StatusOK, listingsPageResponse{
		Items:  toListingResponses(listings, savedMap, seenMap),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
