-- Persisted Telegram (and future multi-channel) delivery ledger.
--
-- Until now nothing recorded whether an alert was actually SENT: listing_history
-- records a *match* (written at scrape/match time, and also by the web browse
-- path, before/independent of delivery), and the only delivery signal was the
-- ephemeral notifier log. This table is the durable source of truth for
-- "was this specific alert delivered to this user, when, how, and did it fail".
--
-- One row per (chat_id, token, alert_type, channel) delivery outcome. A single
-- instant batch (one Telegram message, many listings) inserts N rows sharing a
-- batch_id (the Redis stream message id). Aggregate sends with no per-listing
-- attribution (daily digest) use token = '' and are excluded from the unique
-- index so each send is its own row.
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT      NOT NULL,
    token       TEXT        NOT NULL DEFAULT '',
    search_id   BIGINT      NOT NULL DEFAULT 0,
    alert_type  TEXT        NOT NULL,                 -- instant | price_drop | digest | daily
    channel     TEXT        NOT NULL DEFAULT 'telegram',
    status      TEXT        NOT NULL DEFAULT 'sent',  -- sent | failed | dropped | dead_lettered
    batch_id    TEXT        NOT NULL DEFAULT '',
    attempts    INTEGER     NOT NULL DEFAULT 1,
    error       TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Idempotency: broker reclaim/retry can reprocess the same alert. Collapse a
-- repeated (chat, token, type, channel) outcome to one row (status/attempts
-- updated) instead of inserting duplicate "sent" rows.
CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_deliveries_event
    ON notification_deliveries (chat_id, token, alert_type, channel)
    WHERE token <> '';

-- Per-listing delivered lookup for the admin indicator.
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_token_chat
    ON notification_deliveries (token, chat_id);

-- Per-user delivery feed / audit, newest first.
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_chat_created
    ON notification_deliveries (chat_id, created_at DESC);

-- Reconstruct a batch (one Telegram message -> N listings).
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_batch
    ON notification_deliveries (batch_id) WHERE batch_id <> '';

-- pending_digest stores rendered text only; carry the source tokens so a
-- successful digest flush can attribute the send to specific listings.
ALTER TABLE pending_digest ADD COLUMN IF NOT EXISTS tokens TEXT NOT NULL DEFAULT '';
