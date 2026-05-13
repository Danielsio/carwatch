DROP TABLE IF EXISTS price_list_cache;
ALTER TABLE listing_history DROP COLUMN IF EXISTS base_price;
ALTER TABLE listing_history DROP COLUMN IF EXISTS sub_model_id;
