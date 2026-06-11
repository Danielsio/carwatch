ALTER TABLE search_cycle_stats
  DROP COLUMN IF EXISTS wrong_model,
  DROP COLUMN IF EXISTS year_out,
  DROP COLUMN IF EXISTS price_out,
  DROP COLUMN IF EXISTS km_over,
  DROP COLUMN IF EXISTS hand_over,
  DROP COLUMN IF EXISTS other_filter,
  DROP COLUMN IF EXISTS score_min,
  DROP COLUMN IF EXISTS score_max,
  DROP COLUMN IF EXISTS score_avg,
  DROP COLUMN IF EXISTS price_min,
  DROP COLUMN IF EXISTS price_max;
