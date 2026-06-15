package api

import (
	"net/http"

	"github.com/dsionov/carwatch/internal/storage"
)

// searchActivity returns the daily new-listing counts for a search (sparkline data).
func (s *Server) searchActivity(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	searchID, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}
	days, ok := parseIntParam(w, r, "days", 14)
	if !ok {
		return
	}

	counts, err := s.activity.SearchDailyCounts(r.Context(), chatID, searchID, days)
	if err != nil {
		s.handlerLogger(r, "search_id", searchID).Error("search activity", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if counts == nil {
		counts = []storage.DailyListingCount{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": counts})
}
