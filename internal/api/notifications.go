package api

import "net/http"

type notifCountResponse struct {
	Count int64 `json:"count"`
}

func (s *Server) notificationCount(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())

	since, err := s.notifs.GetLastSeenAt(r.Context(), chatID)
	if err != nil {
		s.logger.Error("get last seen at", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get notification count")
		return
	}

	count, err := s.notifs.CountNewListingsSince(r.Context(), chatID, since)
	if err != nil {
		s.logger.Error("count notifications", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
		return
	}

	writeJSON(w, http.StatusOK, notifCountResponse{Count: count})
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	limit, offset := parsePagination(r)

	since, err := s.notifs.GetLastSeenAt(r.Context(), chatID)
	if err != nil {
		s.logger.Error("get last seen at", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	listings, err := s.notifs.NewListingsSince(r.Context(), chatID, since, limit, offset)
	if err != nil {
		s.logger.Error("list notifications", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	total, err := s.notifs.CountNewListingsSince(r.Context(), chatID, since)
	if err != nil {
		s.logger.Error("count notifications", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
		return
	}

	savedMap := s.savedLookupForRecords(r.Context(), chatID, listings)

	writeJSON(w, http.StatusOK, listingsPageResponse{
		Items:  toListingResponses(listings, savedMap),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Server) markNotificationsSeen(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())

	if err := s.users.UpdateLastSeenAt(r.Context(), chatID); err != nil {
		s.logger.Error("update last seen at", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark as seen")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
