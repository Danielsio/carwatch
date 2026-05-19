package api

import (
	"encoding/json"
	"net/http"

	"github.com/dsionov/carwatch/internal/storage"
)

type subscribeRequest struct {
	Endpoint string        `json:"endpoint"`
	Keys     subscribeKeys `json:"keys"`
}

type subscribeKeys struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}

type unsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

func (s *Server) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" || req.Keys.P256DH == "" || req.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "endpoint, keys.p256dh, and keys.auth are required")
		return
	}

	sub := storage.PushSubscription{
		ChatID:   chatID,
		Endpoint: req.Endpoint,
		P256DH:   req.Keys.P256DH,
		Auth:     req.Keys.Auth,
	}
	if err := s.pushSubs.SavePushSubscription(r.Context(), sub); err != nil {
		s.handlerLogger(r).Error("save push subscription", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}

	var req unsubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "endpoint is required")
		return
	}

	if err := s.pushSubs.DeletePushSubscription(r.Context(), chatID, req.Endpoint); err != nil {
		s.handlerLogger(r).Error("delete push subscription", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pushVAPIDKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"public_key": s.vapidPublicKey,
	})
}
