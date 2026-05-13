ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS sub_model_id INTEGER DEFAULT 0;
ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS base_price INTEGER;

CREATE TABLE IF NOT EXISTS price_list_cache (
    sub_model_id INTEGER NOT NULL,
    year         INTEGER NOT NULL,
    base_price   INTEGER NOT NULL,
    title        TEXT,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sub_model_id, year)
);
