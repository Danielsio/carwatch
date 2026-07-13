-- The browser extension is the primary ingestion path: its Chrome alarm — not
-- the server's maintenance loop — decides when the next real scan happens.
-- Each ingest push self-reports that schedule plus cycle stats, one row per
-- user, and /api/v1/scheduler/status serves it so the web UI's "next scan"
-- countdown ticks toward the same alarm the extension popup shows.
CREATE TABLE IF NOT EXISTS ext_scan_status (
    chat_id          BIGINT PRIMARY KEY,
    started_at       TIMESTAMPTZ NOT NULL,
    next_run_at      TIMESTAMPTZ NOT NULL,
    interval_sec     INTEGER NOT NULL DEFAULT 900,
    searches         INTEGER NOT NULL DEFAULT 0,
    listings_fetched INTEGER NOT NULL DEFAULT 0,
    listings_matched INTEGER NOT NULL DEFAULT 0,
    notifications    INTEGER NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
