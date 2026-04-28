package api

import (
	"net/http"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Server) saveListing(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	if err := s.saved.SaveBookmark(r.Context(), chatID, token); err != nil {
		s.logger.Error("save bookmark", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save bookmark")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unsaveListing(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	if err := s.saved.RemoveBookmark(r.Context(), chatID, token); err != nil {
		s.logger.Error("remove bookmark", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove bookmark")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSaved(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	limit, offset := parsePagination(r)

	listings, err := s.saved.ListSaved(r.Context(), chatID, limit, offset)
	if err != nil {
		s.logger.Error("list saved", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list saved")
		return
	}

	total, err := s.saved.CountSaved(r.Context(), chatID)
	if err != nil {
		s.logger.Error("count saved", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count saved")
		return
	}

	writeJSON(w, http.StatusOK, listingsPageResponse{
		Items:  toListingResponses(listings),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Server) hideListing(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	if err := s.hidden.HideListing(r.Context(), chatID, token); err != nil {
		s.logger.Error("hide listing", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to hide listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unhideListing(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	if err := s.hidden.UnhideListing(r.Context(), chatID, token); err != nil {
		s.logger.Error("unhide listing", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unhide listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listHistory(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	limit, offset := parsePagination(r)

	listings, err := s.listings.ListUserListings(r.Context(), chatID, limit, offset)
	if err != nil {
		s.logger.Error("list history", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list history")
		return
	}

	total, err := s.listings.CountUserListings(r.Context(), chatID)
	if err != nil {
		s.logger.Error("count history", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count history")
		return
	}

	writeJSON(w, http.StatusOK, listingsPageResponse{
		Items:  toListingResponses(listings),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func toListingResponses(records []storage.ListingRecord) []listingResponse {
	items := make([]listingResponse, 0, len(records))
	for _, l := range records {
		items = append(items, listingResponse{
			Token:        l.Token,
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			City:         l.City,
			PageLink:     l.PageLink,
			ImageURL:     l.ImageURL,
			FitnessScore: l.FitnessScore,
			FirstSeenAt:  l.FirstSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return items
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = parseIntParam(r, "limit", 20)
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	offset = parseIntParam(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	return
}
