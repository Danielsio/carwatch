package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
	"github.com/dsionov/carwatch/internal/storage/postgres"
)

func newExtTokenServer(t *testing.T) (*Server, *postgres.Store) {
	t.Helper()
	store := pgtest.NewStore(t)
	srv := New(Config{
		Searches:     store,
		Listings:     store,
		Users:        store,
		Prices:       store,
		Notifs:       store,
		ExtTokens:    store,
		Logger:       slog.Default(),
		FirebaseAuth: &fakeTokenVerifier{uid: "uid-123", email: "user@example.com"},
		API:          config.APIConfig{CORSOrigins: []string{"http://localhost:5173"}},
	})
	return srv, store
}

// mintToken performs the exchange the extension's bridge performs: present the
// short-lived Firebase token once, get a long-lived credential back.
func mintToken(t *testing.T, srv *Server) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/ext/token", strings.NewReader(`{"label":"test browser"}`))
	req.Header.Set("Authorization", "Bearer firebase-token")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("mint token: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp extTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode mint response: %v", err)
	}
	if !strings.HasPrefix(resp.Token, extTokenPrefix) {
		t.Fatalf("minted token has no cwx_ prefix: %q", resp.Token)
	}
	return resp.Token
}

// This is the bug the whole feature exists for: the extension used to borrow the
// web session's Firebase token, which expires in ~1h and which it cannot
// refresh — so scanning silently stopped an hour after the last CarWatch tab
// closed. Its own credential must keep working with no Firebase token in sight.
func TestExtToken_WorksWithoutAnyFirebaseSession(t *testing.T) {
	srv, _ := newExtTokenServer(t)
	token := mintToken(t, srv)

	// The extension's two calls, using ONLY its own credential.
	req := httptest.NewRequest("GET", "/api/v1/ext/searches", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ext/searches with a device token: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/ext/ingest", strings.NewReader(`{"listings":[]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ext/ingest with a device token: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// Scope is structural: the cwx_ credential is accepted by the extension's
// routes and nowhere else, so a stolen token can do what the extension does and
// nothing more.
func TestExtToken_IsRejectedOutsideTheExtensionRoutes(t *testing.T) {
	srv, _ := newExtTokenServer(t)
	token := mintToken(t, srv)

	forbidden := []struct{ method, path string }{
		{"GET", "/api/v1/searches"},        // the web app's route
		{"GET", "/api/v1/me"},              //
		{"DELETE", "/api/v1/searches/1"},   // destructive
		{"GET", "/api/v1/notifications"},   //
		{"GET", "/api/v1/telegram/status"}, //
		{"POST", "/api/v1/ext/token"},      // must never mint another token
		{"DELETE", "/api/v1/ext/token"},    //
		{"GET", "/api/v1/admin/stats"},     //
	}
	for _, tc := range forbidden {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			srv.Routes().ServeHTTP(w, req)

			// Must be an auth rejection specifically — a 404 for a missing
			// resource would mean the token was *accepted* and the route simply
			// had nothing to return, which is still a scope leak.
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("an extension token should be 401 on %s %s, got %d — its scope leaked",
					tc.method, tc.path, w.Code)
			}
		})
	}
}

// Revocation is the answer to a leaked token, so it has to bite immediately.
func TestExtToken_RevocationStopsIngestOnTheNextCall(t *testing.T) {
	srv, _ := newExtTokenServer(t)
	token := mintToken(t, srv)

	req := httptest.NewRequest("DELETE", "/api/v1/ext/token", nil)
	req.Header.Set("Authorization", "Bearer firebase-token") // the user, not the extension
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/v1/ext/searches", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("a revoked token still works: got %d", w.Code)
	}
}

func TestExtToken_UnknownAndExpiredTokensAreRejected(t *testing.T) {
	srv, store := newExtTokenServer(t)

	t.Run("unknown", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ext/searches", nil)
		req.Header.Set("Authorization", "Bearer cwx_never-issued")
		w := httptest.NewRecorder()
		srv.Routes().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("an unknown token was accepted: %d", w.Code)
		}
	})

	t.Run("expired", func(t *testing.T) {
		token, hash, err := newExtToken()
		if err != nil {
			t.Fatal(err)
		}
		// Mint one directly with an expiry in the past.
		chatID, err := store.UpsertWebUser(context.Background(), "uid-expired", "e@x.com")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateExtToken(context.Background(), chatID, hash, "old", time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/api/v1/ext/searches", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		srv.Routes().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("an expired token was accepted: %d", w.Code)
		}
	})
}

// The secret is never stored: a database leak must not yield usable credentials.
func TestExtToken_OnlyTheHashIsPersisted(t *testing.T) {
	srv, store := newExtTokenServer(t)
	token := mintToken(t, srv)

	chatID, err := store.UpsertWebUser(context.Background(), "uid-123", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := store.ListExtTokens(context.Background(), chatID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 stored token, got %d", len(tokens))
	}
	// ExtToken carries no secret field at all — the type cannot even express it.
	// Belt and braces: resolving by the plaintext must fail, only the hash works.
	if _, err := store.ResolveExtToken(context.Background(), token); err == nil {
		t.Fatal("the plaintext token resolved — it is being stored verbatim")
	}
	if _, err := store.ResolveExtToken(context.Background(), hashExtToken(token)); err != nil {
		t.Fatalf("the hash should resolve: %v", err)
	}
}

// Two tokens must never collide, and each must be unguessable.
func TestNewExtToken_IsUniqueAndPrefixed(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, hash, err := newExtToken()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(token, extTokenPrefix) {
			t.Fatalf("token missing prefix: %q", token)
		}
		if len(token) < 40 {
			t.Fatalf("token is too short to be unguessable: %q", token)
		}
		if seen[token] || seen[hash] {
			t.Fatal("duplicate token generated")
		}
		seen[token], seen[hash] = true, true
	}
}

// Connected browsers stay bounded, so a credential from a machine the user no
// longer has cannot linger forever.
func TestExtToken_OldestConnectionIsRevokedBeyondTheCap(t *testing.T) {
	srv, store := newExtTokenServer(t)

	var first string
	for i := 0; i < maxExtTokensPerUser+1; i++ {
		tok := mintToken(t, srv)
		if i == 0 {
			first = tok
		}
	}

	chatID, err := store.UpsertWebUser(context.Background(), "uid-123", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	live, err := store.ListExtTokens(context.Background(), chatID)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) > maxExtTokensPerUser {
		t.Fatalf("connected browsers exceeded the cap: %d", len(live))
	}
	// The oldest one is the one that got dropped.
	if _, err := store.ResolveExtToken(context.Background(), hashExtToken(first)); err == nil {
		t.Fatal("the oldest connection survived past the cap")
	}
}

func TestExtTokenLabel(t *testing.T) {
	if got := extTokenLabel("  Chrome on laptop  "); got != "Chrome on laptop" {
		t.Errorf("label not trimmed: %q", got)
	}
	if got := extTokenLabel("bad\x00label"); strings.ContainsRune(got, 0) {
		t.Errorf("control characters survived: %q", got)
	}
	long := strings.Repeat("א", maxExtTokenLabelRunes+50)
	if got := len([]rune(extTokenLabel(long))); got != maxExtTokenLabelRunes {
		t.Errorf("label not capped on a rune boundary: %d runes", got)
	}
}

var _ storage.ExtTokenStore = (*postgres.Store)(nil)
