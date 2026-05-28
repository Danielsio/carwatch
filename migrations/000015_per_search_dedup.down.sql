-- Revert to per-user dedup by removing duplicates before restoring the old PK.
DELETE FROM seen_listings a
USING seen_listings b
WHERE a.token = b.token AND a.chat_id = b.chat_id
  AND a.ctid > b.ctid;

ALTER TABLE seen_listings DROP CONSTRAINT seen_listings_pkey;
ALTER TABLE seen_listings ADD PRIMARY KEY (token, chat_id);
