package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
)

func TestVitalsRequiresAuthToken(t *testing.T) {
	srv := New(Config{
		Logger: slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
			AuthToken:   "secret-token",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vitals", strings.NewReader(`{"name":"LCP","value":1000}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestVitalsAcceptsAuthorizedRequest(t *testing.T) {
	srv := New(Config{
		Logger: slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
			AuthToken:   "secret-token",
		},
	})

	body := []byte(`{"name":"LCP","value":1234.5,"rating":"good","delta":3.1,"id":"v1","navigationType":"navigate"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vitals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
