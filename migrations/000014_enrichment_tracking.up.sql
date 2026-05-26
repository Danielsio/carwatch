ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS enrich_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS last_enrich_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_listing_history_unenriched
    ON listing_history (first_seen_at DESC)
    WHERE km <= 0;
