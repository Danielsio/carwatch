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
	PriceMax     int    `json:"price_max"`
	EngineMinCC  int    `json:"engine_min_cc"`
	MaxKm        int    `json:"max_km"`
	MaxHand      int    `json:"max_hand"`
	Keywords     string `json:"keywords"`
	ExcludeKeys  string `json:"exclude_keys"`
}

type updateSearchRequest struct {
	YearMin     int    `json:"year_min"`
	YearMax     int    `json:"year_max"`
	PriceMax    int    `json:"price_max"`
	EngineMinCC int    `json:"engine_min_cc"`
	MaxKm       int    `json:"max_km"`
	MaxHand     int    `json:"max_hand"`
	Keywords    string `json:"keywords"`
	ExcludeKeys string `json:"exclude_keys"`
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
	PriceMax         int    `json:"price_max"`
	EngineMinCC      int    `json:"engine_min_cc"`
	MaxKm            int    `json:"max_km"`
	MaxHand          int    `json:"max_hand"`
	Keywords         string `json:"keywords,omitempty"`
	ExcludeKeys      string `json:"exclude_keys,omitempty"`
	Active           bool   `json:"active"`
	CreatedAt        string `json:"created_at"`
	ListingsCount    int64  `json:"listings_count"`
}

var validSources = map[string]bool{
	"yad2":         true,
	"winwin":       true,
	"yad2,winwin":  true,
	"winwin,yad2":  true,
}

func isValidSource(source string) bool {
	return validSources[source]
}

func validateSearchRanges(yearMin, yearMax, priceMax, maxKm, maxHand, engineMinCC int) string {
	if yearMin < 0 {
		return "year_min must not be negative"
	}
	if yearMax < 0 {
		return "year_max must not be negative"
	}
	if yearMin > 0 && yearMax > 0 && yearMin > yearMax {
		return "year_min must not exceed year_max"
	}
	if priceMax < 0 {
		return "price_max must not be negative"
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
	return searchResponse{
		ID:               sr.ID,
		Name:             sr.Name,
		Source:           sr.Source,
		ManufacturerID:   sr.Manufacturer,
		ManufacturerName: s.catalog.ManufacturerName(sr.Manufacturer),
		ModelID:          sr.Model,
		ModelName:        s.catalog.ModelName(sr.Manufacturer, sr.Model),
		YearMin:          sr.YearMin,
		YearMax:          sr.YearMax,
		PriceMax:         sr.PriceMax,
		EngineMinCC:      sr.EngineMinCC,
		MaxKm:            sr.MaxKm,
		MaxHand:          sr.MaxHand,
		Keywords:         sr.Keywords,
		ExcludeKeys:      sr.ExcludeKeys,
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
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	searches, err := s.searches.ListSearches(r.Context(), chatID)
	if err != nil {
		s.logger.Error("list searches", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list searches")
		return
	}

	resp := make([]searchResponse, 0, len(searches))
	for _, sr := range searches {
		resp = append(resp, s.searchResponseWithListingCount(r.Context(), chatID, sr))
	}
	writeJSON(w, http.StatusOK, resp)
}

// validateCreateSearchInput checks catalog IDs, range validation, and duplicate names.
// On success returns (name, 0, ""). On failure returns ("", HTTP status, error message).
func (s *Server) validateCreateSearchInput(ctx context.Context, chatID int64, req *createSearchRequest) (string, int, string) {
	if req.Manufacturer <= 0 || req.Model <= 0 {
		return "", http.StatusBadRequest, "manufacturer and model are required"
	}

	if req.Source == "" {
		req.Source = "yad2"
	}
	if !isValidSource(req.Source) {
		return "", http.StatusBadRequest, "invalid source: must be yad2, winwin, or yad2,winwin"
	}

	if msg := validateSearchRanges(req.YearMin, req.YearMax, req.PriceMax, req.MaxKm, req.MaxHand, req.EngineMinCC); msg != "" {
		return "", http.StatusBadRequest, msg
	}

	mfrName := s.catalog.ManufacturerName(req.Manufacturer)
	if mfrName == "" {
		return "", http.StatusBadRequest, "unknown manufacturer id"
	}
	modelName := s.catalog.ModelName(req.Manufacturer, req.Model)
	if modelName == "" {
		return "", http.StatusBadRequest, "unknown model id"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.ToLower(fmt.Sprintf("%s-%s", mfrName, modelName))
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
		PriceMax:     req.PriceMax,
		EngineMinCC:  req.EngineMinCC,
		MaxKm:        req.MaxKm,
		MaxHand:      req.MaxHand,
		Keywords:     splitKeywords(req.Keywords),
		ExcludeKeys:  splitKeywords(req.ExcludeKeys),
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
		s.logger.Error("create search", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create search")
		return
	}

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

	sr, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		s.logger.Error("get search", "error", err)
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

	existing, err := s.searches.GetSearch(r.Context(), id, chatID)
	if err != nil {
		s.logger.Error("get search for update", "error", err)
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

	if msg := validateSearchRanges(req.YearMin, req.YearMax, req.PriceMax, req.MaxKm, req.MaxHand, req.EngineMinCC); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	existing.YearMin = req.YearMin
	existing.YearMax = req.YearMax
	existing.PriceMax = req.PriceMax
	existing.EngineMinCC = req.EngineMinCC
	existing.MaxKm = req.MaxKm
	existing.MaxHand = req.MaxHand
	existing.Keywords = splitKeywords(req.Keywords)
	existing.ExcludeKeys = splitKeywords(req.ExcludeKeys)

	if err := s.searches.UpdateSearch(r.Context(), *existing); err != nil {
		s.logger.Error("update search", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update search")
		return
	}

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

	if err := s.searches.DeleteSearch(r.Context(), id, chatID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "search not found")
			return
		}
		s.logger.Error("delete search", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete search")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pauseSearch(w http.ResponseWriter, r *http.Request) {
	s.setSearchActive(w, r, false)
}

func (s *Server) resumeSearch(w http.ResponseWriter, r *http.Request) {
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

	if err := s.searches.SetSearchActive(r.Context(), id, chatID, active); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "search not found")
			return
		}
		s.logger.Error("set search active", "error", err, "active", active)
		writeError(w, http.StatusInternalServerError, "failed to update search")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
