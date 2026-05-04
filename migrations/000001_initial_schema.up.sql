-- Initial CarWatch PostgreSQL schema
-- Migrated from SQLite (internal/storage/sqlite/migrate_steps.go)

CREATE TABLE IF NOT EXISTS users (
    chat_id       BIGINT PRIMARY KEY,
    username      TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL DEFAULT '',
    state_data    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    digest_mode   TEXT NOT NULL DEFAULT 'instant',
    digest_interval TEXT NOT NULL DEFAULT '6h',
    digest_last_flushed TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    language      TEXT NOT NULL DEFAULT '',
    tier          TEXT NOT NULL DEFAULT 'free',
    tier_expires_at TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    trial_used    BOOLEAN NOT NULL DEFAULT FALSE,
    daily_digest  BOOLEAN NOT NULL DEFAULT FALSE,
    daily_digest_time TEXT NOT NULL DEFAULT '09:00',
    daily_digest_last_sent TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    channel       TEXT NOT NULL DEFAULT 'telegram',
    channel_id    TEXT NOT NULL DEFAULT '',
    linked_web_id BIGINT,
    last_seen_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_active ON users (active) WHERE active = TRUE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_channel_id ON users (channel, channel_id) WHERE channel_id != '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_telegram_linked_web_id ON users (linked_web_id) WHERE linked_web_id IS NOT NULL AND channel = 'telegram';

CREATE TABLE IF NOT EXISTS searches (
    id            BIGSERIAL PRIMARY KEY,
    chat_id       BIGINT NOT NULL REFERENCES users(chat_id),
    user_seq      INTEGER NOT NULL DEFAULT 0,
    name          TEXT NOT NULL DEFAULT '',
    source        TEXT NOT NULL DEFAULT 'yad2',
    manufacturer  INTEGER NOT NULL DEFAULT 0,
    model         INTEGER NOT NULL DEFAULT 0,
    year_min      INTEGER NOT NULL DEFAULT 0,
    year_max      INTEGER NOT NULL DEFAULT 0,
    price_max     INTEGER NOT NULL DEFAULT 0,
    engine_min_cc INTEGER NOT NULL DEFAULT 0,
    max_km        INTEGER NOT NULL DEFAULT 0,
    max_hand      INTEGER NOT NULL DEFAULT 0,
    keywords      TEXT NOT NULL DEFAULT '',
    exclude_keys  TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    share_token   TEXT
);

CREATE INDEX IF NOT EXISTS idx_searches_active ON searches (active) WHERE active = TRUE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_searches_chat_user_seq ON searches (chat_id, user_seq);
CREATE UNIQUE INDEX IF NOT EXISTS idx_searches_share_token ON searches (share_token) WHERE share_token IS NOT NULL;

CREATE TABLE IF NOT EXISTS seen_listings (
    token         TEXT NOT NULL,
    chat_id       BIGINT NOT NULL,
    search_id     BIGINT NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (token, chat_id)
);

CREATE INDEX IF NOT EXISTS idx_seen_listings_first_seen_at ON seen_listings (first_seen_at);
CREATE INDEX IF NOT EXISTS idx_seen_listings_chatid_firstseen ON seen_listings (chat_id, first_seen_at);
CREATE INDEX IF NOT EXISTS idx_seen_listings_search_chat ON seen_listings (search_id, chat_id);

CREATE TABLE IF NOT EXISTS pending_notifications (
    id            BIGSERIAL PRIMARY KEY,
    recipient     TEXT NOT NULL,
    search_name   TEXT NOT NULL DEFAULT '',
    payload       TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_notifications_created ON pending_notifications (created_at);
CREATE INDEX IF NOT EXISTS idx_pending_notifications_name_recipient ON pending_notifications (search_name, recipient);

CREATE TABLE IF NOT EXISTS price_history (
    id            BIGSERIAL PRIMARY KEY,
    token         TEXT NOT NULL,
    price         INTEGER NOT NULL,
    observed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_price_history_token_observed ON price_history (token, observed_at DESC);

CREATE TABLE IF NOT EXISTS listing_history (
    token         TEXT NOT NULL,
    chat_id       BIGINT NOT NULL,
    search_id     BIGINT NOT NULL DEFAULT 0,
    search_name   TEXT NOT NULL DEFAULT '',
    manufacturer  TEXT NOT NULL DEFAULT '',
    model         TEXT NOT NULL DEFAULT '',
    year          INTEGER NOT NULL DEFAULT 0,
    price         INTEGER NOT NULL DEFAULT 0,
    km            INTEGER NOT NULL DEFAULT 0,
    hand          INTEGER NOT NULL DEFAULT 0,
    city          TEXT NOT NULL DEFAULT '',
    page_link     TEXT NOT NULL DEFAULT '',
    image_url     TEXT NOT NULL DEFAULT '',
    fitness_score DOUBLE PRECISION,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (token, chat_id)
);

CREATE INDEX IF NOT EXISTS idx_listing_history_chat_search ON listing_history (chat_id, search_name);
CREATE INDEX IF NOT EXISTS idx_listing_history_chat_first_seen ON listing_history (chat_id, first_seen_at);
CREATE INDEX IF NOT EXISTS idx_listing_history_token ON listing_history (token);
CREATE INDEX IF NOT EXISTS idx_listing_history_chat_searchid ON listing_history (chat_id, search_id);

CREATE TABLE IF NOT EXISTS pending_digest (
    id            BIGSERIAL PRIMARY KEY,
    chat_id       BIGINT NOT NULL,
    listing_payload TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_digest_chat_created ON pending_digest (chat_id, created_at);

CREATE TABLE IF NOT EXISTS saved_listings (
    chat_id       BIGINT NOT NULL,
    token         TEXT NOT NULL,
    saved_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, token)
);

CREATE TABLE IF NOT EXISTS hidden_listings (
    chat_id       BIGINT NOT NULL,
    token         TEXT NOT NULL,
    hidden_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, token)
);

CREATE TABLE IF NOT EXISTS link_tokens (
    token         TEXT PRIMARY KEY,
    web_chat_id   BIGINT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL,
    used          BOOLEAN NOT NULL DEFAULT FALSE
);
