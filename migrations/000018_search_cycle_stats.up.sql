CREATE TABLE IF NOT EXISTS search_cycle_stats (
    search_id   BIGINT PRIMARY KEY,
    chat_id     BIGINT NOT NULL,
    search_name TEXT NOT NULL,
    cycle_at    TIMESTAMPTZ NOT NULL,
    feed_size   INT NOT NULL DEFAULT 0,
    matched     INT NOT NULL DEFAULT 0,
    new_listings INT NOT NULL DEFAULT 0,
    km_filtered INT NOT NULL DEFAULT 0,
    delivered   INT NOT NULL DEFAULT 0,
    price_drops INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_cycle_stats_chat ON search_cycle_stats(chat_id);
