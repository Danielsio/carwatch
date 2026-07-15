package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
)

// optionalAuthRoutes are the endpoints served behind optionalAuthMiddleware.
var optionalAuthRoutes = []string{
	"/api/v1/searches",
	"/api/v1/me",
	"/api/v1/notifications/count",
}

func newOptionalAuthServer(t *testing.T, verifier TokenVerifier) *Server {
	t.Helper()
	store := pgtest.NewStore(t)
	return New(Config{
		Searches:     store,
		Listings:     store,
		Users:        store,
		Prices:       store,
		Notifs:       store,
		Logger:       slog.Default(),
		FirebaseAuth: verifier,
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
		},
	})
}

// The regression this whole change exists for: an expired or otherwise
// unverifiable token used to be treated as "anonymous", so GET /searches
// answered `200 []`. The extension — whose Firebase token expires about hourly
// — would then scan nothing and report success, and a web client with a broken
// refresh would render an empty dashboard instead of sending the user to log
// in. A credential that does not work must fail loudly.
func TestOptionalAuth_InvalidTokenIsRejectedNotDowngradedToGuest(t *testing.T) {
	srv := newOptionalAuthServer(t, &fakeTokenVerifier{err: errors.New("token expired")})

	for _, route := range optionalAuthRoutes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest("GET", route, nil)
			req.Header.Set("Authorization", "Bearer expired-token")
			w := httptest.NewRecorder()
			srv.Routes().ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 for an expired token, got %d: %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get("WWW-Authenticate"); got == "" {
				t.Error("expected a WWW-Authenticate header telling the client to re-authenticate")
			}
		})
	}
}

// A malformed Authorization header is still an attempt to authenticate: it must
// not be silently downgraded to a guest response either.
func TestOptionalAuth_MalformedAuthHeaderIsRejected(t *testing.T) {
	srv := newOptionalAuthServer(t, &fakeTokenVerifier{uid: "uid-123", email: "user@example.com"})

	for _, hdr := range []string{"Basic dXNlcjpwYXNz", "Bearer", "garbage"} {
		t.Run(hdr, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/searches", nil)
			req.Header.Set("Authorization", hdr)
			w := httptest.NewRecorder()
			srv.Routes().ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 for Authorization %q, got %d", hdr, w.Code)
			}
		})
	}
}

// The other half of the contract: a caller offering NO credential is a guest,
// not a failure. The public landing and try-search flows depend on this.
func TestOptionalAuth_NoCredentialStillGetsGuestResponse(t *testing.T) {
	srv := newOptionalAuthServer(t, &fakeTokenVerifier{uid: "uid-123", email: "user@example.com"})

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 guest response with no Authorization header, got %d: %s", w.Code, w.Body.String())
	}

	var searches []any
	if err := json.Unmarshal(w.Body.Bytes(), &searches); err != nil {
		t.Fatalf("guest response is not a JSON list: %v (%s)", err, w.Body.String())
	}
	if len(searches) != 0 {
		t.Fatalf("expected an empty guest search list, got %d entries", len(searches))
	}
}

// A valid token keeps working, of course.
func TestOptionalAuth_ValidTokenIsAuthenticated(t *testing.T) {
	srv := newOptionalAuthServer(t, &fakeTokenVerifier{uid: "uid-123", email: "user@example.com"})

	req := httptest.NewRequest("GET", "/api/v1/searches", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with a valid token, got %d: %s", w.Code, w.Body.String())
	}
}

// Dev auth (no Firebase configured) follows the same rule: a wrong dev token is
// a failed authentication, while no header at all is the anonymous dev user.
func TestOptionalAuth_DevToken(t *testing.T) {
	store := pgtest.NewStore(t)
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

	t.Run("wrong dev token is rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/searches", nil)
		req.Header.Set("Authorization", "Bearer not-the-dev-secret")
		w := httptest.NewRecorder()
		srv.Routes().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for a wrong dev token, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("correct dev token is accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/searches", nil)
		req.Header.Set("Authorization", "Bearer dev-secret")
		w := httptest.NewRecorder()
		srv.Routes().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for the correct dev token, got %d: %s", w.Code, w.Body.String())
		}
	})
}
