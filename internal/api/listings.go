package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/timeutil"
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

type ingestRequest struct {
	Listings []ingestListing `json:"listings"`
}

type ingestListing struct {
	Token          string  `json:"token"`
	Manufacturer   string  `json:"manufacturer"`
	ManufacturerID int     `json:"manufacturer_id"`
	Model          string  `json:"model"`
	ModelID        int     `json:"model_id"`
	SubModel       string  `json:"sub_model"`
	SubModelID     int     `json:"sub_model_id"`
	BodyType       string  `json:"body_type"`
	Year           int     `json:"year"`
	Price          int     `json:"price"`
	Km             int     `json:"km"`
	Hand           int     `json:"hand"`
	City           string  `json:"city"`
	Area           string  `json:"area"`
	ImageURL       string  `json:"image_url"`
	PageLink       string  `json:"page_link"`
	EngineVolume   float64 `json:"engine_volume"`
	EngineType     string  `json:"engine_type"`
	GearBox        string  `json:"gear_box"`
	Description    string  `json:"description"`
	IsCommercial   *bool   `json:"is_commercial"`
	CreatedAt      string  `json:"created_at"`
}

type ingestResponse struct {
	Processed  int `json:"processed"`
	NewMatches int `json:"new_matches"`
}

func (s *Server) ingestListings(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}

	log := s.handlerLogger(r, "op", "ingest")

	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(req.Listings) == 0 {
		writeJSON(w, http.StatusOK, ingestResponse{})
		return
	}

	if len(req.Listings) > 500 {
		writeError(w, http.StatusBadRequest, "too many listings (max 500)")
		return
	}

	raw := make([]model.RawListing, 0, len(req.Listings))
	for _, l := range req.Listings {
		if l.Token == "" {
			continue
		}
		rl := model.RawListing{
			Token:          l.Token,
			Manufacturer:   l.Manufacturer,
			ManufacturerID: l.ManufacturerID,
			Model:          l.Model,
			ModelID:        l.ModelID,
			SubModel:       l.SubModel,
			SubModelID:     l.SubModelID,
			BodyType:       l.BodyType,
			Year:           l.Year,
			Price:          l.Price,
			Km:             l.Km,
			Hand:           l.Hand,
			City:           l.City,
			Area:           l.Area,
			ImageURL:       l.ImageURL,
			PageLink:       l.PageLink,
			EngineVolume:   l.EngineVolume,
			EngineType:     l.EngineType,
			GearBox:        l.GearBox,
			Description:    l.Description,
			Commercial:     l.IsCommercial,
		}
		// Shared with the scraper: the Yad2 feed emits zone-less local
		// timestamps, so parsing them as UTC would put posted_at ~2-3h early.
		rl.CreatedAt = timeutil.ParseFlexTime(l.CreatedAt)
		raw = append(raw, rl)
	}

	// How many of the pushed listings arrived with km — the key signal when
	// debugging "extension enriched but km didn't land" (if this is high but
	// nothing saves with km, the loss is at the per-search filter/save below).
	parsedWithKm := 0
	for i := range raw {
		if raw[i].Km > 0 {
			parsedWithKm++
		}
	}

	searches, err := s.searches.ListSearches(r.Context(), chatID)
	if err != nil {
		log.Error("list searches failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list searches")
		return
	}

	totalNew := 0
	for _, sr := range searches {
		if !sr.Active || (sr.Manufacturer == 0 && sr.Model == 0) {
			continue
		}

		criteria := model.FilterCriteriaFromSearch(&sr)
		filtered := filter.Apply(criteria, raw)
		if len(filtered) == 0 {
			continue
		}

		matchedWithKm := 0
		for i := range filtered {
			if filtered[i].Km > 0 {
				matchedWithKm++
			}
		}

		params := scheduler.ProcessParamsFromSearch(sr, nil)
		params.ChatID = chatID
		pr := s.pipeline.Process(r.Context(), filtered, params)
		if len(pr.Records) > 0 {
			if err := s.listings.SaveListings(r.Context(), pr.Records); err != nil {
				log.Error("save listings failed", "search_id", sr.ID, "error", err)
				continue
			}
			totalNew += len(pr.Records)

			// Notify the user about genuinely new matches (dedup-gated) via the
			// same delivery path the scheduler uses. Best-effort; never fails
			// the ingest, which has already persisted the listings.
			s.deliverIngestMatches(r.Context(), sr, pr.Listings, log)
		}

		// NOTE: removal/reconciliation (deleting a search's listings absent from
		// the feed, as the retired scraper did) is intentionally NOT done here.
		// The extension pushes page 1 of each feed, and the backend can't tell a
		// complete feed from a paginated/partial one (the per-search filter
		// decouples `filtered` from the raw feed size), so reconciling could
		// delete valid listings. Safe removal needs the extension to signal
		// per-search feed completeness — tracked as a follow-up.

		// Per-search visibility: how many pushed listings matched this search,
		// how many of those carried km, and how many were persisted. A gap
		// between matched_with_km and saved points at the filter/pipeline.
		log.Debug("ingest search processed",
			"search_id", sr.ID, "search_name", sr.Name,
			"matched", len(filtered), "matched_with_km", matchedWithKm,
			"saved", len(pr.Records))
	}

	log.Info("ingest complete",
		"submitted", len(req.Listings), "parsed_with_km", parsedWithKm,
		"parsed", len(raw), "new_matches", totalNew)

	writeJSON(w, http.StatusOK, ingestResponse{
		Processed:  len(raw),
		NewMatches: totalNew,
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
