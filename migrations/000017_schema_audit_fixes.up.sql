-- Add missing FK on push_subscriptions to cascade user deletion.
ALTER TABLE push_subscriptions
  ADD CONSTRAINT fk_push_subs_user
  FOREIGN KEY (chat_id) REFERENCES users(chat_id) ON DELETE CASCADE;

-- Drop stale index on search_name (all queries use search_id).
DROP INDEX IF EXISTS idx_listing_history_chat_search;

-- Replace narrow unenriched index (km-only) with broader partial index
-- covering all enrichment criteria checked by the enricher worker.
DROP INDEX IF EXISTS idx_listing_history_unenriched;
CREATE INDEX idx_listing_history_unenriched
  ON listing_history (first_seen_at DESC)
  WHERE (km <= 0 OR COALESCE(city, '') = '' OR COALESCE(image_url, '') = '')
    AND enrich_attempts < 10;
