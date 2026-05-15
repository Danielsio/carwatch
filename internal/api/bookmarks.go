package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dsionov/carwatch/internal/storage"
)

const (
	maxSavedListings  = 500
	maxHiddenListings = 1000
)

func (s *Server) saveListing(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	log := s.handlerLogger(r, "op", "save_bookmark", "token", token)
	count, err := s.saved.CountSaved(r.Context(), chatID)
	if err != nil {
		log.Error("count saved failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save bookmark")
		return
	}
	if count >= maxSavedListings {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("הגעת למגבלת %d מודעות שמורות", maxSavedListings))
		return
	}
	if err := s.saved.SaveBookmark(r.Context(), chatID, token); err != nil {
		log.Error("save bookmark failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save bookmark")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unsaveListing(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	log := s.handlerLogger(r, "op", "remove_bookmark", "token", token)
	if err := s.saved.RemoveBookmark(r.Context(), chatID, token); err != nil {
		log.Error("remove bookmark failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove bookmark")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSaved(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(w, r)
	if !ok {
		return
	}

	log := s.handlerLogger(r, "op", "list_saved")

	listings, err := s.saved.ListSaved(r.Context(), chatID, limit, offset)
	if err != nil {
		log.Error("list saved failed", "limit", limit, "offset", offset, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list saved")
		return
	}

	total, err := s.saved.CountSaved(r.Context(), chatID)
	if err != nil {
		log.Error("count saved failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count saved")
		return
	}

	savedMap := make(map[string]bool, len(listings))
	for _, l := range listings {
		savedMap[l.Token] = true
	}
	seenMap := s.seenLookupForRecords(r.Context(), chatID, listings)

	writeJSON(w, http.StatusOK, listingsPageResponse{
		Items:  toListingResponses(listings, savedMap, seenMap),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Server) hideListing(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	log := s.handlerLogger(r, "op", "hide_listing", "token", token)
	count, err := s.hidden.CountHidden(r.Context(), chatID)
	if err != nil {
		log.Error("count hidden failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to hide listing")
		return
	}
	if count >= maxHiddenListings {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("הגעת למגבלת %d מודעות מוסתרות", maxHiddenListings))
		return
	}
	if err := s.hidden.HideListing(r.Context(), chatID, token); err != nil {
		log.Error("hide listing failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to hide listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unhideListing(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	log := s.handlerLogger(r, "op", "unhide_listing", "token", token)
	if err := s.hidden.UnhideListing(r.Context(), chatID, token); err != nil {
		log.Error("unhide listing failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unhide listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listHistory(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(w, r)
	if !ok {
		return
	}

	log := s.handlerLogger(r, "op", "list_history")

	listings, err := s.listings.ListUserListings(r.Context(), chatID, limit, offset)
	if err != nil {
		log.Error("list history failed", "limit", limit, "offset", offset, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list history")
		return
	}

	total, err := s.listings.CountUserListings(r.Context(), chatID)
	if err != nil {
		log.Error("count history failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count history")
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

func (s *Server) savedLookupForRecords(ctx context.Context, chatID int64, records []storage.ListingRecord) map[string]bool {
	if s.saved == nil || len(records) == 0 {
		return nil
	}
	tokens := make([]string, len(records))
	for i, l := range records {
		tokens[i] = l.Token
	}
	m, err := s.saved.SavedAmong(ctx, chatID, tokens)
	if err != nil {
		s.logger.Error("saved among lookup failed", "chat_id", chatID, "tokens", len(tokens), "error", err)
		return nil
	}
	return m
}

func (s *Server) seenLookupForRecords(ctx context.Context, chatID int64, records []storage.ListingRecord) map[string]bool {
	if s.notifs == nil || len(records) == 0 {
		return nil
	}
	tokens := make([]string, len(records))
	for i, l := range records {
		tokens[i] = l.Token
	}
	m, err := s.notifs.ListingUserSeenAmong(ctx, chatID, tokens)
	if err != nil {
		s.logger.Error("seen among lookup failed", "chat_id", chatID, "tokens", len(tokens), "error", err)
		return nil
	}
	return m
}

func toListingResponses(records []storage.ListingRecord, saved, seen map[string]bool) []listingResponse {
	items := make([]listingResponse, 0, len(records))
	for _, l := range records {
		savedFlag := false
		if saved != nil && saved[l.Token] {
			savedFlag = true
		}
		seenFlag := false
		if seen != nil && seen[l.Token] {
			seenFlag = true
		}
		items = append(items, listingResponse{
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
	return items
}

func parsePagination(w http.ResponseWriter, r *http.Request) (limit, offset int, ok bool) {
	limit, ok = parseIntParam(w, r, "limit", 20)
	if !ok {
		return
	}
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	offset, ok = parseIntParam(w, r, "offset", 0)
	if !ok {
		return
	}
	if offset < 0 {
		offset = 0
	}
	return
}
