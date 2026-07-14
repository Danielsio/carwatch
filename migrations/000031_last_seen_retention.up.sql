-- Retention must be keyed on when a listing was LAST seen, not when it was
-- first seen.
--
-- The extension re-pushes every listing that still matches a search on every
-- 15-minute cycle, and cars sit on Yad2 for months. Pruning a dedup claim by
-- first_seen_at therefore expired the claim of a listing that was still very
-- much alive: the next push re-claimed it as new and the user was re-alerted
-- about a car they had already seen — every storage.prune_after (30d) for as
-- long as the ad stayed up. listing_history had the same shape at 90d: a live
-- listing's row was deleted and re-inserted, resetting first_seen_at (so it
-- resurfaced as "new" in the UI) along with its enrichment counters.
--
-- last_seen_at is refreshed on every re-observation (dedup claim / listing
-- upsert), so retention now measures "gone from the source for N days", which
-- is what the retention window was always meant to mean.

ALTER TABLE seen_listings
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE listing_history
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Existing rows: the only evidence we have of their last observation is when
-- they were first seen. Seeding from first_seen_at keeps the new prune's
-- behaviour identical to the old one for rows nothing has re-observed yet, and
-- the next ingest cycle refreshes everything that is still live.
UPDATE seen_listings SET last_seen_at = first_seen_at;
UPDATE listing_history SET last_seen_at = first_seen_at;

-- The prune is the only reader of seen_listings.first_seen_at, and it now reads
-- last_seen_at instead; move the index with it. (The composite
-- idx_seen_listings_chatid_firstseen is left alone — different query shape.)
DROP INDEX IF EXISTS idx_seen_listings_first_seen_at;
CREATE INDEX IF NOT EXISTS idx_seen_listings_last_seen_at ON seen_listings (last_seen_at);
