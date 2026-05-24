CREATE TABLE IF NOT EXISTS cycle_log (
    id              BIGSERIAL PRIMARY KEY,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_ms     INTEGER NOT NULL DEFAULT 0,
    searches        INTEGER NOT NULL DEFAULT 0,
    listings_fetched INTEGER NOT NULL DEFAULT 0,
    listings_matched INTEGER NOT NULL DEFAULT 0,
    notifications    INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'ok'
);

CREATE INDEX idx_cycle_log_started_at ON cycle_log (started_at DESC);
