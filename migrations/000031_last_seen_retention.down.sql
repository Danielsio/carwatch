DROP INDEX IF EXISTS idx_seen_listings_last_seen_at;
CREATE INDEX IF NOT EXISTS idx_seen_listings_first_seen_at ON seen_listings (first_seen_at);

ALTER TABLE listing_history DROP COLUMN IF EXISTS last_seen_at;
ALTER TABLE seen_listings DROP COLUMN IF EXISTS last_seen_at;
