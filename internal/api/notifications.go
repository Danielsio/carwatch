package api

import "net/http"

type notifCountResponse struct {
	Count int64 `json:"count"`
}

func (s *Server) notificationCount(w http.ResponseWriter, r *http.Request) {
	chatID, ok := chatIDFromContext(r.Context())
	if !ok {
		// Guest user — return zero count.
		writeJSON(w, http.StatusOK, notifCountResponse{Count: 0})
		return
	}

	log := s.handlerLogger(r, "op", "notification_count")

	since, err := s.notifs.GetLastSeenAt(r.Context(), chatID)
	if err != nil {
		log.Error("get last seen at failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get notification count")
		return
	}

	count, err := s.notifs.CountNewListingsSince(r.Context(), chatID, since)
	if err != nil {
		log.Error("count notifications failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
		return
	}

	writeJSON(w, http.StatusOK, notifCountResponse{Count: count})
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(w, r)
	if !ok {
		return
	}

	log := s.handlerLogger(r, "op", "list_notifications")

	since, err := s.notifs.GetLastSeenAt(r.Context(), chatID)
	if err != nil {
		log.Error("get last seen at failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	listings, err := s.notifs.NewListingsSince(r.Context(), chatID, since, limit, offset)
	if err != nil {
		log.Error("list notifications failed", "limit", limit, "offset", offset, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	total, err := s.notifs.CountNewListingsSince(r.Context(), chatID, since)
	if err != nil {
		log.Error("count notifications failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
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

func (s *Server) markNotificationsSeen(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}

	log := s.handlerLogger(r, "op", "mark_notifications_seen")
	if err := s.users.UpdateLastSeenAt(r.Context(), chatID); err != nil {
		log.Error("update last seen at failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark as seen")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
