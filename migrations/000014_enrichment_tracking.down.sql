DROP INDEX IF EXISTS idx_listing_history_unenriched;
ALTER TABLE listing_history DROP COLUMN IF EXISTS last_enrich_at;
ALTER TABLE listing_history DROP COLUMN IF EXISTS enrich_attempts;
