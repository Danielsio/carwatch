ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS median_price INTEGER;
ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS cohort_size INTEGER;
ALTER TABLE listing_history ADD COLUMN IF NOT EXISTS deal_score INTEGER;
