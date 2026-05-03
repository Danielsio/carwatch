package api

import (
	"fmt"
	"net/http"
	"strings"
)

func normalizeBotUsername(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), "@")
}

func (s *Server) postTelegramLink(w http.ResponseWriter, r *http.Request) {
	if s.linkTokens == nil {
		writeError(w, http.StatusServiceUnavailable, "telegram link unavailable")
		return
	}
	botUser := normalizeBotUsername(s.botUsername)
	if botUser == "" {
		s.logger.Error("telegram link: bot username not configured")
		writeError(w, http.StatusServiceUnavailable, "telegram bot not configured")
		return
	}

	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	token, err := s.linkTokens.CreateLinkToken(r.Context(), chatID)
	if err != nil {
		s.logger.Error("create link token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	link := fmt.Sprintf("https://t.me/%s?start=link_%s", botUser, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"link":               link,
		"expires_in_seconds": 900,
	})
}

func (s *Server) getTelegramStatus(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	tgUser, err := s.users.GetLinkedTelegramUser(r.Context(), chatID)
	if err != nil {
		s.logger.Error("get linked telegram user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := map[string]any{
		"connected":         tgUser != nil,
		"telegram_username": nil,
	}
	if tgUser != nil && tgUser.Username != "" {
		resp["telegram_username"] = tgUser.Username
	}
	writeJSON(w, http.StatusOK, resp)
}
