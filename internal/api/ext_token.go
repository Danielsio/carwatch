package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// extTokenPrefix marks a CarWatch extension credential. It exists so the auth
// middleware can tell one from a Firebase ID token by inspection, without a
// database probe on every request.
const extTokenPrefix = "cwx_"

// extTokenTTL is the initial lifetime. It slides forward while the token is in
// use (see ResolveExtToken), so an extension that keeps scanning keeps working,
// while one that stops — uninstalled, abandoned profile — ages out by itself.
const extTokenTTL = 90 * 24 * time.Hour

// maxExtTokensPerUser bounds how many browsers one account can have connected.
// Beyond it the oldest connection is revoked, so a stale token from a machine
// the user no longer has cannot linger forever.
const maxExtTokensPerUser = 5

// newExtToken mints a credential and returns it with its hash. The plaintext
// exists exactly once — in the response to the user — and is never stored:
// a leak of the database must not yield a usable credential.
func newExtToken() (token, hash string, err error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", "", err
	}
	token = extTokenPrefix + base64.RawURLEncoding.EncodeToString(buf[:])
	return token, hashExtToken(token), nil
}

func hashExtToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isExtToken(bearer string) bool {
	return strings.HasPrefix(bearer, extTokenPrefix)
}

// maxExtTokenLabelRunes caps the user-supplied label ("Chrome on my laptop").
const maxExtTokenLabelRunes = 100

// extTokenLabel trims a submitted label, strips control characters, and caps it
// on a rune boundary — the label is user-supplied text that gets rendered back.
func extTokenLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(s))

	runes := []rune(s)
	if len(runes) > maxExtTokenLabelRunes {
		return string(runes[:maxExtTokenLabelRunes])
	}
	return s
}

type extTokenResponse struct {
	// Token is the plaintext credential. It is returned exactly once, at
	// creation, and cannot be recovered afterwards.
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type extTokenInfo struct {
	ID         int64   `json:"id"`
	Label      string  `json:"label"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	ExpiresAt  string  `json:"expires_at"`
}

// createExtToken issues a credential for the caller's browser extension.
//
// It sits behind the strict (Firebase) auth middleware: proving you are the
// user is exactly what earns you a long-lived token. The extension calls this
// once, with the short-lived Firebase token it captured while a CarWatch tab
// was open, and from then on uses only what it gets back — which is why it
// keeps working long after that tab is closed.
func (s *Server) createExtToken(w http.ResponseWriter, r *http.Request) {
	if s.extTokens == nil {
		writeError(w, http.StatusServiceUnavailable, "extension tokens unavailable")
		return
	}
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	log := s.handlerLogger(r, "op", "ext_token_create")

	var body struct {
		Label string `json:"label"`
	}
	// A body is optional; a malformed one is not worth failing over.
	_ = json.NewDecoder(r.Body).Decode(&body)
	label := extTokenLabel(body.Label)

	// Keep the number of connected browsers bounded, so a token from a machine
	// the user no longer has cannot linger indefinitely.
	if existing, err := s.extTokens.ListExtTokens(r.Context(), chatID); err == nil && len(existing) >= maxExtTokensPerUser {
		if n, err := s.extTokens.RevokeOldestExtTokens(r.Context(), chatID, maxExtTokensPerUser-1); err != nil {
			log.Error("failed to revoke oldest ext tokens", "error", err)
		} else if n > 0 {
			log.Info("revoked oldest extension connections to stay within the cap",
				"revoked", n, "cap", maxExtTokensPerUser)
		}
	}

	token, hash, err := newExtToken()
	if err != nil {
		log.Error("failed to generate ext token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	expiresAt := time.Now().Add(extTokenTTL)
	if _, err := s.extTokens.CreateExtToken(r.Context(), chatID, hash, label, expiresAt); err != nil {
		log.Error("failed to store ext token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Info("issued extension token", "expires_at", expiresAt.UTC())
	writeJSON(w, http.StatusOK, extTokenResponse{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

// listExtTokensHandler reports the caller's connected browsers. Metadata only —
// the secrets are not stored and cannot be shown again.
func (s *Server) listExtTokensHandler(w http.ResponseWriter, r *http.Request) {
	if s.extTokens == nil {
		writeError(w, http.StatusServiceUnavailable, "extension tokens unavailable")
		return
	}
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}

	tokens, err := s.extTokens.ListExtTokens(r.Context(), chatID)
	if err != nil {
		s.handlerLogger(r, "op", "ext_token_list").Error("failed to list ext tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]extTokenInfo, 0, len(tokens))
	for _, t := range tokens {
		info := extTokenInfo{
			ID:        t.ID,
			Label:     t.Label,
			CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: t.ExpiresAt.UTC().Format(time.RFC3339),
		}
		if t.LastUsedAt != nil {
			used := t.LastUsedAt.UTC().Format(time.RFC3339)
			info.LastUsedAt = &used
		}
		items = append(items, info)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// revokeExtTokens disconnects every browser. This is the answer to a leaked
// token, and it takes effect on the extension's very next call.
func (s *Server) revokeExtTokens(w http.ResponseWriter, r *http.Request) {
	if s.extTokens == nil {
		writeError(w, http.StatusServiceUnavailable, "extension tokens unavailable")
		return
	}
	chatID, ok := s.requireResolvedChatID(w, r)
	if !ok {
		return
	}
	log := s.handlerLogger(r, "op", "ext_token_revoke")

	n, err := s.extTokens.RevokeExtTokens(r.Context(), chatID)
	if err != nil {
		log.Error("failed to revoke ext tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	log.Info("revoked extension tokens", "revoked", n)
	writeJSON(w, http.StatusOK, map[string]any{"revoked": n})
}

// extAuthMiddleware authenticates the extension's own routes.
//
// A CarWatch extension credential (cwx_…) is accepted here and NOWHERE else:
// scoping is structural rather than an allowlist someone has to remember to
// maintain. So a stolen extension token can do exactly what the extension does
// — read the searches to scan and push listings back — and nothing else. It
// cannot delete a search, read notifications, or touch an admin route.
//
// Anything that is not a cwx_ token falls through to the normal (Firebase/dev)
// auth, so a freshly installed extension can still reach these routes with the
// Firebase token it captured, which is how it bootstraps its own credential.
func (s *Server) extAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer := bearerFromAuthHeader(r.Header.Get("Authorization"))
		if !isExtToken(bearer) {
			s.authMiddleware(next).ServeHTTP(w, r)
			return
		}
		if s.extTokens == nil {
			writeAuthError(w, "extension tokens unavailable")
			return
		}

		chatID, err := s.extTokens.ResolveExtToken(r.Context(), hashExtToken(bearer))
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				s.logger.Error("resolve ext token failed", "error", err)
			}
			// Revoked, expired, or never existed — all the same to the caller,
			// and all mean "re-connect the extension".
			writeAuthError(w, "invalid or revoked extension token")
			return
		}

		ctx := withChatID(r.Context(), chatID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withChatID injects an authenticated chat id (and an empty email: a device
// token identifies a browser, not a login session).
func withChatID(ctx context.Context, chatID int64) context.Context {
	ctx = context.WithValue(ctx, chatIDKey, chatID)
	return context.WithValue(ctx, emailKey, "")
}
