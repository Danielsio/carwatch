-- The browser extension needs a credential of its own.
--
-- Until now it borrowed the web session's Firebase ID token, captured by
-- monkey-patching window.fetch on the CarWatch site. Firebase ID tokens expire
-- in about an hour and the extension has no way to refresh one, so roughly an
-- hour after the user closed their last CarWatch tab the extension's token went
-- stale — and, because the extension IS the only path listings enter the system
-- by, scanning and notifications simply stopped. The product's core promise held
-- only while a tab happened to be open somewhere.
--
-- An ext_token is a long-lived, revocable credential minted for one browser.
-- Only its SHA-256 hash is stored: a database leak must not yield a usable
-- credential, and there is never a reason to read the token back.

CREATE TABLE IF NOT EXISTS ext_tokens (
    id           BIGSERIAL PRIMARY KEY,
    chat_id      BIGINT NOT NULL REFERENCES users(chat_id) ON DELETE CASCADE,
    -- SHA-256 hex of the token. Unique so a lookup is a single index probe and
    -- the same token can never be minted twice.
    token_hash   TEXT NOT NULL UNIQUE,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    -- Sliding expiry: refreshed while the token is in use, so an extension that
    -- keeps scanning keeps working, while one that stops (uninstalled browser,
    -- abandoned profile) ages out on its own.
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ext_tokens_chat ON ext_tokens (chat_id);
