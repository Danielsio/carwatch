package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dsionov/carwatch/internal/storage"
)

type createSearchRequest struct {
	Name         string `json:"name"`
	Source       string `json:"source"`
	Manufacturer int    `json:"manufacturer"`
	Model        int    `json:"model"`
	YearMin      int    `json:"year_min"`
	YearMax      int    `json:"year_max"`
	PriceMin     int    `json:"price_min"`
	PriceMax     int    `json:"price_max"`
	EngineMinCC  int    `json:"engine_min_cc"`
	MaxKm        int    `json:"max_km"`
	MaxHand      int    `json:"max_hand"`
	Keywords     string `json:"keywords"`
	ExcludeKeys  string `json:"exclude_keys"`
	// SellerFilter: any (default), private, commercial (also accepts dealer, dealership for commercial).
	SellerFilter string `json:"seller_filter,omitempty"`
	GearBox      string `json:"gear_box,omitempty"`
	PriceOnly    bool   `json:"price_only,omitempty"`
	PhotoOnly    bool   `json:"photo_only,omitempty"`
}

type updateSearchRequest struct {
	YearMin      int     `json:"year_min"`
	YearMax      int     `json:"year_max"`
	PriceMin     int     `json:"price_min"`
	PriceMax     int     `json:"price_max"`
	EngineMinCC  int     `json:"engine_min_cc"`
	MaxKm        int     `json:"max_km"`
	MaxHand      int     `json:"max_hand"`
	Keywords     string  `json:"keywords"`
	ExcludeKeys  string  `json:"exclude_keys"`
	SellerFilter *string `json:"seller_filter,omitempty"`
	GearBox      *string `json:"gear_box,omitempty"`
	PriceOnly    *bool   `json:"price_only,omitempty"`
	PhotoOnly    *bool   `json:"photo_only,omitempty"`
}

type searchResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Source           string `json:"source"`
	ManufacturerID   int    `json:"manufacturer_id"`
	ManufacturerName string `json:"manufacturer_name"`
	ModelID          int    `json:"model_id"`
	ModelName        string `json:"model_name"`
	YearMin          int    `json:"year_min"`
	YearMax          int    `json:"year_max"`
	PriceMin         int    `json:"price_min,omitempty"`
	PriceMax         int    `json:"price_max"`
	EngineMinCC      int    `json:"engine_min_cc"`
	MaxKm            int    `json:"max_km"`
	MaxHand          int    `json:"max_hand"`
	Keywords         string `json:"keywords,omitempty"`
	ExcludeKeys      string `json:"exclude_keys,omitempty"`
	SellerFilter     string `json:"seller_filter,omitempty"`
	GearBox          string `json:"gear_box,omitempty"`
	PriceOnly        bool   `json:"price_only,omitempty"`
	PhotoOnly        bool   `json:"photo_only,omitempty"`
	Active           bool   `json:"active"`
	CreatedAt        string `json:"created_at"`
	ListingsCount    int64  `json:"listings_count"`
}

func (s *Server) ensureUserActive(ctx context.Context, chatID int64) {
	user, err := s.users.GetUser(ctx, chatID)
	if err != nil || user == nil {
		return
	}
	if !user.Active {
		if err := s.users.SetUserActive(ctx, chatID, true); err != nil {
			s.logger.Error("reactivate user on search create", "chat_id", chatID, "error", err)
			return
		}
		s.logger.Info("reactivated inactive user on search action", "chat_id", chatID)
	}
}

func isValidSource(source string) bool {
	switch source {
	case "yad2":
		return true
	default:
		return false
	}
}

func validateSearchRanges(yearMin, yearMax, priceMin, priceMax, maxKm, maxHand, engineMinCC int) string {
	if yearMin < 0 {
		return "year_min must not be negative"
	}
	if yearMax < 0 {
		return "year_max must not be negative"
	}
	if yearMin > 0 && yearMax > 0 && yearMin > yearMax {
		return "year_min must not exceed year_max"
	}
	if priceMin < 0 {
		return "price_min must not be negative"
	}
	if priceMax < 0 {
		return "price_max must not be negative"
	}
	if priceMin > 0 && priceMax > 0 && priceMin > priceMax {
		return "price_min must not exceed price_max"
	}
	if maxKm < 0 {
		return "max_km must not be negative"
	}
	if maxHand < 0 {
		return "max_hand must not be negative"
	}
	if engineMinCC < 0 {
		return "engine_min_cc must not be negative"
	}
	return ""
}

func (s *Server) toSearchResponse(sr storage.Search) searchResponse {
	mfrName := "כל היצרנים"
	if sr.Manufacturer > 0 {
		mfrName = s.catalog.ManufacturerName(sr.Manufacturer)
	}
	mdlName := "כל הדגמים"
	if sr.Model > 0 {
		mdlName = s.catalog.ModelName(sr.Manufacturer, sr.Model)
	}
	return searchResponse{
		ID:               sr.ID,
		Name:             sr.Name,
		Source:           sr.Source,
		ManufacturerID:   sr.Manufacturer,
		ManufacturerName: mfrName,
		ModelID:          sr.Model,
		ModelName:        mdlName,
		YearMin:          sr.YearMin,
		YearMax:          sr.YearMax,
		PriceMin:         sr.PriceMin,
		PriceMax:         sr.PriceMax,
		EngineMinCC:      sr.EngineMinCC,
		MaxKm:            sr.MaxKm,
		MaxHand:          sr.MaxHand,
		Keywords:         sr.Keywords,
		ExcludeKeys:      sr.ExcludeKeys,
		SellerFilter:     storage.NormalizeSellerFilter(sr.SellerFilter),
		GearBox:          sr.GearBox,
		PriceOnly:        sr.PriceOnly,
		PhotoOnly:        sr.PhotoOnly,
		Active:           sr.Active,
		CreatedAt:        sr.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) searchResponseWithListingCount(ctx context.Context, chatID int64, sr storage.Search) searchResponse {
	item := s.toSearchResponse(sr)
	if s.listings != nil {
		n, err := s.listings.CountSearchListings(ctx, chatID, sr.ID, listingFilterFromSearch(&sr))
		if err != nil {
			s.logger.Error("count search listings", "error", err, "search_id", sr.ID)
		} else {
			item.ListingsCount = n
		}
	}
	return item
}

func (s *Server) listSearches(w http.ResponseWriter, r *http.Request) {
	chatID, ok := chatIDFromContext(r.Context())
	if !ok {
		// Guest user — return empty list.
		writeJSON(w, http.StatusOK, []searchResponse{})
		return
	}
	log := s.handlerLogger(r, "op", "list_searches")

	searches, err := s.searches.ListSearches(r.Context(), chatID)
	if err != nil {
		log.Error("list searches failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list searches")
		return
	}

	var counts map[int64]int64
	if s.listings != nil {
		var err error
		counts, err = s.listings.CountSearchListingsForChat(r.Context(), chatID)
		if err != nil {
			log.Error("count search listings batch failed", "error", err)
			counts = nil
		}
	}

	resp := make([]searchResponse, 0, len(searches))
	for _, sr := range searches {
		if counts != nil {
			item := s.toSearchResponse(sr)
			item.ListingsCount = counts[sr.ID]
			resp = append(resp, item)
			continue
		}
		resp = append(resp, s.searchResponseWithListingCount(r.Context(), chatID, sr))
	}
	writeJSON(w, http.StatusOK, resp)
}

// validateCreateSearchInput checks catalog IDs, range validation, and duplicate names.
// Manufacturer and model are both optional (0 = all).
// On success returns (name, 0, ""). On failure returns ("", HTTP status, error message).
func (s *Server) validateCreateSearchInput(ctx context.Context, chatID int64, req *createSearchRequest) (string, int, string) {
	if req.Manufacturer < 0 || req.Model < 0 {
		return "", http.StatusBadRequest, "manufacturer and model must not be negative"
	}

	if req.Source == "" {
		req.Source = "yad2"
	}
	if !isValidSource(req.Source) {
		return "", http.StatusBadRequest, "invalid source: must be yad2"
	}

	if msg := validateSearchRanges(req.YearMin, req.YearMax, req.PriceMin, req.PriceMax, req.MaxKm, req.MaxHand, req.EngineMinCC); msg != "" {
		return "", http.StatusBadRequest, msg
	}

	var mfrName, modelName string
	if req.Manufacturer > 0 {
		mfrName = s.catalog.ManufacturerName(req.Manufacturer)
		if mfrName == "" || mfrName == "Unknown" {
			return "", http.StatusBadRequest, "unknown manufacturer id"
		}
	}
	if req.Model > 0 {
		if req.Manufacturer <= 0 {
			return "", http.StatusBadRequest, "model requires a manufacturer"
		}
		modelName = s.catalog.ModelName(req.Manufacturer, req.Model)
		if modelName == "" || modelName == "Unknown" {
			return "", http.StatusBadRequest, "unknown model id"
		}
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		switch {
		case req.Manufacturer == 0:
			name = "all-cars"
		case req.Model == 0:
			name = strings.ToLower(mfrName) + "-all"
		default:
			name = strings.ToLower(fmt.Sprintf("%s-%s", mfrName, modelName))
		}
	}

	existing, err := s.searches.ListSearches(ctx, chatID)
	if err != nil {
		s.logger.Error("list searches for duplicate check", "error", err)
		return "", http.StatusInternalServerError, "failed to validate search name"
	}
	for _, ex := range existing {
		if strings.EqualFold(strings.TrimSpace(ex.Name), name) {
			return "", http.StatusConflict, "search name already exists"
		}
	}
	return name, 0, ""
}

func createSearchRecord(chatID int64, name string, req createSearchRequest) storage.Search {
	return storage.Search{
		ChatID:       chatID,
		Name:         name,
		Source:       req.Source,
		Manufacturer: req.Manufacturer,
		Model:        req.Model,
		YearMin:      req.YearMin,
		YearMax:      req.YearMax,
		PriceMin:     req.PriceMin,
		PriceMax:     req.PriceMax,
		EngineMinCC:  req.EngineMinCC,
		MaxKm:        req.MaxKm,
		MaxHand:      req.MaxHand,
		Keywords:     splitKeywords(req.Keywords),
		ExcludeKeys:  splitKeywords(req.ExcludeKeys),
		SellerFilter: storage.NormalizeSellerFilter(req.SellerFilter),
		GearBox:      req.GearBox,
		PriceOnly:    req.PriceOnly,
		PhotoOnly:    req.PhotoOnly,
		Active:       true,
	}
}

func (s *Server) writeCreatedSearch(w http.ResponseWriter, r *http.Request, chatID, id int64) {
	created, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil || created == nil {
		s.logger.Error("get created search", "error", err)
		writeError(w, http.StatusInternalServerError, "search created but failed to retrieve")
		return
	}

	writeJSON(w, http.StatusCreated, s.searchResponseWithListingCount(r.Context(), chatID, *created))

	if s.poller != nil {
		s.poller.TriggerPoll()
	}
}

func (s *Server) createSearch(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}

	log := s.handlerLogger(r, "op", "create_search")

	if s.cfg.MaxSearches > 0 {
		count, err := s.searches.CountSearches(r.Context(), chatID)
		if err != nil {
			log.Error("count searches for limit check failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to check search limit")
			return
		}
		if count >= int64(s.cfg.MaxSearches) {
			writeError(w, http.StatusConflict, "search limit reached")
			return
		}
	}

	var req createSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name, st, msg := s.validateCreateSearchInput(r.Context(), chatID, &req)
	if st != 0 {
		writeError(w, st, msg)
		return
	}

	id, err := s.searches.CreateSearch(r.Context(), createSearchRecord(chatID, name, req))
	if err != nil {
		log.Error("create search failed", "name", name,
			"manufacturer", req.Manufacturer, "model", req.Model, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create search")
		return
	}

	log.Info("search created", "search_id", id, "name", name,
		"manufacturer", req.Manufacturer, "model", req.Model)
	s.ensureUserActive(r.Context(), chatID)
	s.writeCreatedSearch(w, r, chatID, id)
}

func (s *Server) getSearch(w http.ResponseWriter, r *http.Request) {
	chatID, okChat := requireChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "get_search", "search_id", id)

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

	writeJSON(w, http.StatusOK, s.searchResponseWithListingCount(r.Context(), chatID, *sr))
}

func (s *Server) updateSearch(w http.ResponseWriter, r *http.Request) {
	chatID, okChat := requireChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "update_search", "search_id", id)

	existing, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		log.Error("get search for update failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	var req updateSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateSearchRanges(req.YearMin, req.YearMax, req.PriceMin, req.PriceMax, req.MaxKm, req.MaxHand, req.EngineMinCC); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	existing.YearMin = req.YearMin
	existing.YearMax = req.YearMax
	existing.PriceMin = req.PriceMin
	existing.PriceMax = req.PriceMax
	existing.EngineMinCC = req.EngineMinCC
	existing.MaxKm = req.MaxKm
	existing.MaxHand = req.MaxHand
	existing.Keywords = splitKeywords(req.Keywords)
	existing.ExcludeKeys = splitKeywords(req.ExcludeKeys)
	if req.SellerFilter != nil {
		existing.SellerFilter = storage.NormalizeSellerFilter(*req.SellerFilter)
	}
	if req.GearBox != nil {
		existing.GearBox = *req.GearBox
	}
	if req.PriceOnly != nil {
		existing.PriceOnly = *req.PriceOnly
	}
	if req.PhotoOnly != nil {
		existing.PhotoOnly = *req.PhotoOnly
	}

	if err := s.searches.UpdateSearch(r.Context(), *existing); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "search not found")
			return
		}
		log.Error("update search failed", "search_name", existing.Name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update search")
		return
	}
	log.Info("search updated", "search_name", existing.Name)

	writeJSON(w, http.StatusOK, s.searchResponseWithListingCount(r.Context(), chatID, *existing))
}

func (s *Server) deleteSearch(w http.ResponseWriter, r *http.Request) {
	chatID, okChat := requireChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "delete_search", "search_id", id)

	if err := s.searches.DeleteSearch(r.Context(), id, chatID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "search not found")
			return
		}
		log.Error("delete search failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete search")
		return
	}

	log.Info("search deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) searchStats(w http.ResponseWriter, r *http.Request) {
	chatID, okChat := requireChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "search_stats", "search_id", id)

	sr, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		log.Error("get search for stats failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if sr == nil {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	if s.listings == nil {
		writeError(w, http.StatusServiceUnavailable, "listing store not available")
		return
	}

	stats, err := s.listings.SearchStats(r.Context(), chatID, id, listingFilterFromSearch(sr))
	if err != nil {
		log.Error("search stats query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) pauseSearch(w http.ResponseWriter, r *http.Request) {
	s.setSearchActive(w, r, false)
}

func (s *Server) resumeSearch(w http.ResponseWriter, r *http.Request) {
	chatID, ok := chatIDFromContext(r.Context())
	if ok {
		s.ensureUserActive(r.Context(), chatID)
	}
	s.setSearchActive(w, r, true)
}

func (s *Server) setSearchActive(w http.ResponseWriter, r *http.Request, active bool) {
	chatID, okChat := requireChatID(w, r)
	if !okChat {
		return
	}
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	log := s.handlerLogger(r, "op", "set_search_active", "search_id", id, "active", active)

	if err := s.searches.SetSearchActive(r.Context(), id, chatID, active); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "search not found")
			return
		}
		log.Error("set search active failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update search")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
