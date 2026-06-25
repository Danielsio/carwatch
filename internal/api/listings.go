package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage"
)

type listingResponse struct {
	Token             string                  `json:"token"`
	SearchName        string                  `json:"search_name,omitempty"`
	Manufacturer      string                  `json:"manufacturer"`
	Model             string                  `json:"model"`
	SubModel          string                  `json:"sub_model,omitempty"`
	BodyType          string                  `json:"body_type,omitempty"`
	Year              int                     `json:"year"`
	Price             int                     `json:"price"`
	Km                int                     `json:"km"`
	Hand              int                     `json:"hand"`
	City              string                  `json:"city"`
	PageLink          string                  `json:"page_link"`
	ImageURL          string                  `json:"image_url,omitempty"`
	EngineVolume      float64                 `json:"engine_volume,omitempty"`
	HorsePower        int                     `json:"horse_power,omitempty"`
	EngineType        string                  `json:"engine_type,omitempty"`
	GearBox           string                  `json:"gear_box,omitempty"`
	Description       string                  `json:"description,omitempty"`
	FitnessScore      *float64                `json:"fitness_score,omitempty"`
	ScoreBreakdown    *scoreBreakdownResponse `json:"score_breakdown,omitempty"`
	OriginalOwnership *string                 `json:"original_ownership,omitempty"`
	MedianPrice       *int                    `json:"median_price,omitempty"`
	CohortSize        *int                    `json:"cohort_size,omitempty"`
	DealScore         *int                    `json:"deal_score,omitempty"`
	BasePrice         *int                    `json:"base_price,omitempty"`
	FirstSeenAt       string                  `json:"first_seen_at"`
	PostedAt          string                  `json:"posted_at,omitempty"`
	Saved             bool                    `json:"saved,omitempty"`
	// Seen: user dismissed this listing from the new-items feed (notifications).
	Seen bool `json:"seen,omitempty"`
	// IsCommercial: omitted when unknown; false = private seller; true = dealer/commercial.
	IsCommercial *bool `json:"is_commercial,omitempty"`
	// RemovedAt: set when the listing disappeared from the source but is bookmarked.
	RemovedAt *string `json:"removed_at,omitempty"`
	// SuspiciousReasons: reasons the listing was flagged as suspicious.
	SuspiciousReasons []string `json:"suspicious_reasons,omitempty"`
}

type scoreBreakdownResponse struct {
	Condition float64 `json:"condition"`
	Value     float64 `json:"value"`
	Engine    float64 `json:"engine"`
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

	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "refresh", "search_id", id)

	s.sweepRefreshCooldowns()

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
		log.Error("get search failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if sr == nil {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	log = log.With("search_name", sr.Name)

	source := sr.Source
	if source == "" {
		source = "yad2"
	}
	sources := strings.Split(source, ",")
	var allRaw []model.RawListing
	var fetchSucceeded bool
	var lastFetchErr error
	for _, src := range sources {
		src = strings.TrimSpace(src)
		f, ok := s.fetchers.Get(src)
		if !ok {
			continue
		}
		params := model.SourceParamsFromSearch(sr)

		var raw []model.RawListing
		var fetchErr error
	retryLoop:
		for attempt := 0; attempt < 3; attempt++ {
			raw, fetchErr = f.Fetch(r.Context(), params)
			if fetchErr == nil {
				break
			}
			log.Warn("fetch attempt failed", "source", src,
				"attempt", fmt.Sprintf("%d/3", attempt+1), "error", fetchErr)
			if attempt < 2 {
				delay := time.Duration(1<<attempt) * time.Second
				select {
				case <-r.Context().Done():
					fetchErr = r.Context().Err()
					break retryLoop
				case <-time.After(delay):
				}
			}
		}
		if fetchErr != nil {
			lastFetchErr = fetchErr
			continue
		}
		fetchSucceeded = true
		allRaw = append(allRaw, raw...)
	}

	if !fetchSucceeded {
		log.Error("all fetch attempts failed, keeping existing listings",
			"sources", source, "error", lastFetchErr)
		writeError(w, http.StatusBadGateway, "failed to fetch listings from source")
		return
	}

	s.refreshMu.Store(cooldownKey, time.Now())

	criteria := model.FilterCriteriaFromSearch(sr)
	filtered := filter.Apply(criteria, allRaw)

	freshTokens := make([]string, len(filtered))
	for i, l := range filtered {
		freshTokens[i] = l.Token
	}

	removed, err := s.listings.DeleteStaleListings(r.Context(), chatID, sr.ID, freshTokens)
	if err != nil {
		log.Error("delete stale listings failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to clean stale listings")
		return
	}

	// Run the shared pipeline to score, enrich, and build ListingRecords.
	// The API does not have a market cache, so deal scoring is skipped (nil).
	params := scheduler.ProcessParamsFromSearch(*sr, nil)
	params.ChatID = chatID // override with the authenticated user's chat ID
	pr := s.pipeline.Process(r.Context(), filtered, params)
	if len(pr.Records) > 0 {
		if err := s.listings.SaveListings(r.Context(), pr.Records); err != nil {
			log.Error("save listings failed", "records", len(pr.Records), "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save listings")
			return
		}
	}

	lf := listingFilterFromSearch(sr)
	listings, err := s.listings.ListSearchListings(r.Context(), chatID, sr.ID, lf, 100, 0, "newest")
	if err != nil {
		log.Error("list listings failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list listings")
		return
	}
	total, err := s.listings.CountSearchListings(r.Context(), chatID, sr.ID, lf)
	if err != nil {
		log.Error("count listings failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count listings")
		return
	}

	log.Info("refresh complete",
		"fetched", len(allRaw), "filtered", len(filtered),
		"removed", removed, "total", total)

	savedMap := s.savedLookupForRecords(r.Context(), chatID, listings)
	seenMap := s.seenLookupForRecords(r.Context(), chatID, listings)

	writeJSON(w, http.StatusOK, refreshResponse{
		Items:   toListingResponses(listings, savedMap, seenMap),
		Total:   total,
		Removed: removed,
	})
}

func (s *Server) getListing(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	log := s.handlerLogger(r, "op", "get_listing", "token", token)

	l, err := s.listings.GetListing(r.Context(), chatID, token)
	if err != nil {
		log.Error("get listing failed", "error", err)
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
			log.Error("check saved status failed", "error", err)
		}
	}

	seenFlag := false
	seenMap := s.seenLookupForRecords(r.Context(), chatID, []storage.ListingRecord{{Token: token}})
	if seenMap != nil && seenMap[token] {
		seenFlag = true
	}

	resp := listingResponse{
		Token:        l.Token,
		SearchName:   l.SearchName,
		Manufacturer: l.Manufacturer,
		Model:        l.Model,
		SubModel:     l.SubModel,
		BodyType:     l.BodyType,
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
	}
	if l.ScoreCondition != nil && l.ScoreValue != nil && l.ScoreEngine != nil {
		resp.ScoreBreakdown = &scoreBreakdownResponse{
			Condition: *l.ScoreCondition,
			Value:     *l.ScoreValue,
			Engine:    *l.ScoreEngine,
		}
	}
	if l.OriginalOwnership != nil && *l.OriginalOwnership != "" {
		resp.OriginalOwnership = l.OriginalOwnership
	}
	if l.PostedAt != nil {
		resp.PostedAt = l.PostedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if l.RemovedAt != nil {
		s := l.RemovedAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.RemovedAt = &s
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) listingPriceHistory(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.requireResolvedChatID(w, r)
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
		writeError(w, http.StatusInternalServerError, "failed to verify listing ownership")
		return
	}
	if l == nil {
		writeError(w, http.StatusNotFound, "listing not found")
		return
	}

	points, err := s.prices.GetPriceHistory(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get price history")
		return
	}

	type priceRecord struct {
		Price      int    `json:"price"`
		ObservedAt string `json:"observed_at"`
	}
	items := make([]priceRecord, 0, len(points))
	for _, p := range points {
		items = append(items, priceRecord{
			Price:      p.Price,
			ObservedAt: p.ObservedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func listingFilterFromSearch(sr *storage.Search) storage.ListingFilter {
	f := storage.ListingFilter{
		PriceMin:  sr.PriceMin,
		PriceMax:  sr.PriceMax,
		YearMin:   sr.YearMin,
		YearMax:   sr.YearMax,
		MaxKm:     sr.MaxKm,
		MaxHand:   sr.MaxHand,
		GearBox:   sr.GearBox,
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

const (
	refreshCooldownEvictAge = 2 * time.Minute // entries older than this are evicted
	refreshSweepInterval    = 1 * time.Minute // sweep at most this often
)

// sweepRefreshCooldowns removes stale entries from refreshMu. It runs at most
// once per refreshSweepInterval to avoid overhead on every request.
func (s *Server) sweepRefreshCooldowns() {
	now := time.Now()
	lastNs := s.lastRefreshSweep.Load()
	if lastNs > 0 && now.Sub(time.Unix(0, lastNs)) < refreshSweepInterval {
		return
	}
	// CAS to avoid concurrent sweeps; if we lose the race, skip.
	if !s.lastRefreshSweep.CompareAndSwap(lastNs, now.UnixNano()) {
		return
	}
	cutoff := now.Add(-refreshCooldownEvictAge)
	s.refreshMu.Range(func(key, value any) bool {
		if ts, ok := value.(time.Time); ok && ts.Before(cutoff) {
			s.refreshMu.Delete(key)
		}
		return true
	})
}

func (s *Server) listListings(w http.ResponseWriter, r *http.Request) {
	chatID, okChat := s.requireResolvedChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "list_listings", "search_id", id)

	sr, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		log.Error("get search failed", "error", err)
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
		log.Error("list listings failed", "search_name", sr.Name,
			"sort", sort, "limit", limit, "offset", offset, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list listings")
		return
	}

	total, err := s.listings.CountSearchListings(r.Context(), chatID, sr.ID, f)
	if err != nil {
		log.Error("count listings failed", "search_name", sr.Name, "error", err)
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
