package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func TestAuthMiddleware_FirebaseValid(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srv := New(Config{
		Searches:     store,
		Listings:     store,
		Users:        store,
		Prices:       store,
		Logger:       slog.Default(),
		FirebaseAuth: &fakeTokenVerifier{uid: "uid-123", email: "user@example.com"},
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	// Should succeed (200) since token verification passes
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid Firebase token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_FirebaseInvalidToken(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srv := New(Config{
		Searches:     store,
		Listings:     store,
		Users:        store,
		Prices:       store,
		Logger:       slog.Default(),
		FirebaseAuth: &fakeTokenVerifier{err: errors.New("invalid token")},
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
		},
	})

	// Use a strict-auth endpoint; GET /api/v1/searches uses optional auth now.
	req := httptest.NewRequest("GET", "/api/v1/telegram/status", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid Firebase token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_FirebaseNoBearer(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srv := New(Config{
		Searches:     store,
		Listings:     store,
		Users:        store,
		Prices:       store,
		Logger:       slog.Default(),
		FirebaseAuth: &fakeTokenVerifier{uid: "uid-123", email: "user@example.com"},
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
		},
	})

	// Use a strict-auth endpoint; GET /api/v1/searches uses optional auth now.
	req := httptest.NewRequest("GET", "/api/v1/telegram/status", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_DevChatID_ValidToken(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.UpsertUser(context.Background(), 42, "testuser"); err != nil {
		t.Fatal(err)
	}

	srv := New(Config{
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   42,
			AuthToken:   "dev-secret",
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	req.Header.Set("Authorization", "Bearer dev-secret")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid dev token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_DevChatID_NoToken(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srv := New(Config{
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   42,
			AuthToken:   "dev-secret",
		},
	})

	// Use a strict-auth endpoint; GET /api/v1/searches uses optional auth now.
	req := httptest.NewRequest("GET", "/api/v1/telegram/status", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.UpsertUser(context.Background(), 999, "testuser"); err != nil {
		t.Fatal(err)
	}

	srv := New(Config{
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options=DENY, got %q", w.Header().Get("X-Frame-Options"))
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options=nosniff, got %q", w.Header().Get("X-Content-Type-Options"))
	}
	if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy=strict-origin-when-cross-origin, got %q", w.Header().Get("Referrer-Policy"))
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.UpsertUser(context.Background(), 999, "testuser"); err != nil {
		t.Fatal(err)
	}

	srv := New(Config{
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	reqID := w.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header to be set")
	}
	if len(reqID) < 8 {
		t.Errorf("expected X-Request-ID to be at least 8 chars, got %q", reqID)
	}
}

func TestCORSMiddleware_AllowedMethods(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srv := New(Config{
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Logger:   slog.Default(),
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			DevChatID:   999,
		},
	})

	req := httptest.NewRequest("OPTIONS", "/api/v1/searches", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}

	headers := w.Header().Get("Access-Control-Allow-Headers")
	if headers == "" {
		t.Error("expected Access-Control-Allow-Headers header")
	}

	maxAge := w.Header().Get("Access-Control-Max-Age")
	if maxAge != "86400" {
		t.Errorf("expected Access-Control-Max-Age=86400, got %q", maxAge)
	}

	vary := w.Header().Get("Vary")
	if vary != "Origin" {
		t.Errorf("expected Vary=Origin, got %q", vary)
	}
}
