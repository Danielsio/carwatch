package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dsionov/carwatch/internal/storage"
)

type searchRequest struct {
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
		CreatedAt:        sr.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) listSearches(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	searches, err := s.searches.ListSearches(r.Context(), chatID)
	if err != nil {
		s.logger.Error("list searches", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list searches")
		return
	}

	resp := make([]searchResponse, 0, len(searches))
	for _, sr := range searches {
		resp = append(resp, s.toSearchResponse(sr))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) createSearch(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Manufacturer <= 0 || req.Model <= 0 {
		writeError(w, http.StatusBadRequest, "manufacturer and model are required")
		return
	}

	if req.Source == "" {
		req.Source = "yad2"
	}

	mfrName := s.catalog.ManufacturerName(req.Manufacturer)
	modelName := s.catalog.ModelName(req.Manufacturer, req.Model)
	name := strings.ToLower(fmt.Sprintf("%s-%s", mfrName, modelName))

	search := storage.Search{
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

	id, err := s.searches.CreateSearch(r.Context(), search)
	if err != nil {
		s.logger.Error("create search", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create search")
		return
	}

	created, err := s.searches.GetSearch(r.Context(), id)
	if err != nil {
		s.logger.Error("get created search", "error", err)
		writeError(w, http.StatusInternalServerError, "search created but failed to retrieve")
		return
	}

	writeJSON(w, http.StatusCreated, s.toSearchResponse(*created))
}

func (s *Server) getSearch(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	sr, err := s.searches.GetSearch(r.Context(), id)
	if err != nil {
		s.logger.Error("get search", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if sr == nil || sr.ChatID != chatID {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	writeJSON(w, http.StatusOK, s.toSearchResponse(*sr))
}

func (s *Server) updateSearch(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	existing, err := s.searches.GetSearch(r.Context(), id)
	if err != nil {
		s.logger.Error("get search for update", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if existing == nil || existing.ChatID != chatID {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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

	writeJSON(w, http.StatusOK, s.toSearchResponse(*existing))
}

func (s *Server) deleteSearch(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	if err := s.searches.DeleteSearch(r.Context(), id, chatID); err != nil {
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
	chatID := chatIDFromContext(r.Context())
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	if err := s.searches.SetSearchActive(r.Context(), id, chatID, active); err != nil {
		s.logger.Error("set search active", "error", err, "active", active)
		writeError(w, http.StatusInternalServerError, "failed to update search")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
