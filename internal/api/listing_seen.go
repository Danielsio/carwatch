package api

import "net/http"

func (s *Server) markListingSeen(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}
	log := s.handlerLogger(r, "op", "mark_listing_seen", "token", token)
	if err := s.notifs.MarkListingUserSeen(r.Context(), chatID, token); err != nil {
		log.Error("mark listing seen failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unmarkListingSeen(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}
	log := s.handlerLogger(r, "op", "unmark_listing_seen", "token", token)
	if err := s.notifs.UnmarkListingUserSeen(r.Context(), chatID, token); err != nil {
		log.Error("unmark listing seen failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unmark listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
