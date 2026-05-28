ALTER TABLE seen_listings DROP CONSTRAINT seen_listings_pkey;
ALTER TABLE seen_listings ADD PRIMARY KEY (token, chat_id, search_id);
