package api

import "testing"

func TestConfirmTokenStore_SingleUse(t *testing.T) {
	store := newConfirmTokenStore()
	const admin = int64(1)

	tok, _, err := store.issue(admin)
	if err != nil {
		t.Fatal(err)
	}
	if !store.consume(admin, tok) {
		t.Fatal("a freshly issued token should be accepted once")
	}
	// Burned: a replay (the "misclick sends it twice" case) must not work.
	if store.consume(admin, tok) {
		t.Fatal("a confirm token was accepted twice — replay is possible")
	}
}

func TestConfirmTokenStore_WrongTokenBurnsTheOutstandingOne(t *testing.T) {
	store := newConfirmTokenStore()
	const admin = int64(1)

	tok, _, _ := store.issue(admin)
	// A wrong attempt must fail AND invalidate the outstanding token, so a
	// destructive call always starts from a fresh, deliberate confirmation.
	if store.consume(admin, "not-the-token") {
		t.Fatal("a wrong token was accepted")
	}
	if store.consume(admin, tok) {
		t.Fatal("the real token still worked after a wrong attempt burned it")
	}
}

func TestConfirmTokenStore_ExpiredTokenRejected(t *testing.T) {
	store := newConfirmTokenStore()
	const admin = int64(1)

	tok, _, _ := store.issue(admin)
	// Force expiry.
	store.mu.Lock()
	entry := store.tokens[admin]
	entry.expires = entry.expires.Add(-2 * confirmTokenTTL)
	store.tokens[admin] = entry
	store.mu.Unlock()

	if store.consume(admin, tok) {
		t.Fatal("an expired confirm token was accepted")
	}
}

func TestConfirmTokenStore_IsPerAdmin(t *testing.T) {
	store := newConfirmTokenStore()
	tok1, _, _ := store.issue(1)
	// Admin 2 cannot use admin 1's token.
	if store.consume(2, tok1) {
		t.Fatal("one admin's confirm token was usable by another")
	}
	// Admin 1's own token still works (admin 2's failed attempt didn't burn it).
	if !store.consume(1, tok1) {
		t.Fatal("admin 1's token was consumed by admin 2's attempt")
	}
}

func TestConfirmTokenStore_EmptyTokenNeverConsumes(t *testing.T) {
	store := newConfirmTokenStore()
	if _, _, err := store.issue(1); err != nil {
		t.Fatal(err)
	}
	if store.consume(1, "") {
		t.Fatal("an empty confirm token was accepted")
	}
}
