package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// The destructive admin endpoints — reset-all and purge — delete production
// data in a single call, recoverable only from the nightly backup. One misclick
// in the admin UI, or a replayed request, and the data is gone. A confirm token
// turns that into a deliberate two-step action: the client fetches a token,
// then must echo it back within a short window on the destructive call.
//
// The store is in-memory and single-use. Tokens are short-lived, so a lost or
// leaked one expires on its own, and a process restart simply invalidates any
// outstanding tokens — which fails safe (a destructive call must re-confirm).

const (
	confirmTokenTTL      = 60 * time.Second
	confirmTokenMaxPerIP = 32 // bound the map against a token-minting flood
)

type confirmToken struct {
	token   string
	expires time.Time
}

type confirmTokenStore struct {
	mu     sync.Mutex
	tokens map[int64]confirmToken // adminChatID -> outstanding token
}

func newConfirmTokenStore() *confirmTokenStore {
	return &confirmTokenStore{tokens: make(map[int64]confirmToken)}
}

// issue mints a fresh confirm token for an admin, replacing any outstanding one.
func (c *confirmTokenStore) issue(chatID int64) (string, time.Time, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", time.Time{}, err
	}
	tok := hex.EncodeToString(buf[:])
	expires := time.Now().Add(confirmTokenTTL)

	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistically drop expired entries so an admin churning tokens cannot
	// grow the map without bound.
	if len(c.tokens) > confirmTokenMaxPerIP {
		now := time.Now()
		for id, t := range c.tokens {
			if now.After(t.expires) {
				delete(c.tokens, id)
			}
		}
	}
	c.tokens[chatID] = confirmToken{token: tok, expires: expires}
	return tok, expires, nil
}

// consume validates and burns a token: it returns true at most once per issued
// token, and only within its TTL.
func (c *confirmTokenStore) consume(chatID int64, tok string) bool {
	if tok == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	stored, ok := c.tokens[chatID]
	if !ok {
		return false
	}
	// Single-use regardless of outcome: a wrong or expired attempt burns it, so
	// a destructive call always starts from a fresh, deliberate confirmation.
	delete(c.tokens, chatID)
	if time.Now().After(stored.expires) {
		return false
	}
	return subtleEqual(stored.token, tok)
}

// subtleEqual is a constant-time-ish comparison; the tokens are short and
// random, so timing is not a realistic vector, but there is no reason to leak.
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// adminConfirmToken mints a confirm token for the destructive admin endpoints.
func (s *Server) adminConfirmToken(w http.ResponseWriter, r *http.Request) {
	chatID, ok := requireChatID(w, r)
	if !ok {
		return
	}
	tok, expires, err := s.confirmTokens.issue(chatID)
	if err != nil {
		s.handlerLogger(r, "op", "admin_confirm_token").Error("failed to mint confirm token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"confirm_token":      tok,
		"expires_in_seconds": int(time.Until(expires).Seconds()),
	})
}

// requireConfirmToken checks the confirm token supplied on a destructive call,
// writing a 428 (Precondition Required) when it is missing or invalid so the
// client knows to fetch one and retry. It also emits the audit line: who did
// what is worth recording distinctly from the operation's own success log.
func (s *Server) requireConfirmToken(w http.ResponseWriter, r *http.Request, confirmTok, op string) bool {
	chatID, ok := chatIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid user context")
		return false
	}
	if !s.confirmTokens.consume(chatID, confirmTok) {
		s.handlerLogger(r, "op", op).Warn("destructive admin op refused: missing or invalid confirm token",
			"actor_chat_id", chatID, "email", emailFromContext(r.Context()))
		writeError(w, http.StatusPreconditionRequired,
			"this action requires a fresh confirm token (GET /api/v1/admin/confirm-token)")
		return false
	}
	// Audit: a destructive action was deliberately confirmed. Kept as a distinct,
	// greppable line — actor, action — separate from the op's result log.
	s.handlerLogger(r, "op", op).Warn("AUDIT destructive admin op confirmed",
		"actor_chat_id", chatID, "email", emailFromContext(r.Context()))
	return true
}
