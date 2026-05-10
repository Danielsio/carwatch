CREATE TABLE IF NOT EXISTS listing_user_seen (
    chat_id  BIGINT NOT NULL REFERENCES users (chat_id) ON DELETE CASCADE,
    token    TEXT NOT NULL,
    seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, token)
);

CREATE INDEX IF NOT EXISTS idx_listing_user_seen_chat ON listing_user_seen (chat_id);
