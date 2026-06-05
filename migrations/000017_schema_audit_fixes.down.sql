-- Remove FK on push_subscriptions.
ALTER TABLE push_subscriptions DROP CONSTRAINT IF EXISTS fk_push_subs_user;

-- Restore original stale index on search_name.
CREATE INDEX IF NOT EXISTS idx_listing_history_chat_search
  ON listing_history (chat_id, search_name);

-- Restore narrow unenriched index (km-only).
DROP INDEX IF EXISTS idx_listing_history_unenriched;
CREATE INDEX idx_listing_history_unenriched
  ON listing_history (first_seen_at DESC)
  WHERE km <= 0;
