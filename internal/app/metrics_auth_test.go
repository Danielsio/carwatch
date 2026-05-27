package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsAuthMiddleware_LocalBindAllowsWithoutToken(t *testing.T) {
	called := false
	h := metricsAuthMiddleware("127.0.0.1:8080", "", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}

func TestMetricsAuthMiddleware_NonLocalRequiresToken(t *testing.T) {
	h := metricsAuthMiddleware("0.0.0.0:8080", "secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMetricsAuthMiddleware_NonLocalAcceptsTelemetryHeader(t *testing.T) {
	h := metricsAuthMiddleware("0.0.0.0:8080", "secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("X-CarWatch-Telemetry-Token", "secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMetricsAuthMiddleware_NonLocalAcceptsBearerToken(t *testing.T) {
	h := metricsAuthMiddleware("0.0.0.0:8080", "secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
