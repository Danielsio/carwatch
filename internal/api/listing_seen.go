package api

import "net/http"

func (s *Server) markListingSeen(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}
	if err := s.notifs.MarkListingUserSeen(r.Context(), chatID, token); err != nil {
		s.logger.Error("mark listing seen", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unmarkListingSeen(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}
	if err := s.notifs.UnmarkListingUserSeen(r.Context(), chatID, token); err != nil {
		s.logger.Error("unmark listing seen", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unmark listing")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
