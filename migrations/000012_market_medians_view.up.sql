CREATE MATERIALIZED VIEW IF NOT EXISTS market_medians AS
SELECT
    manufacturer,
    model,
    year,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY price) AS median_price,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY NULLIF(km, 0)) AS median_km,
    COUNT(*) AS cohort_size
FROM listing_history
WHERE manufacturer IS NOT NULL
  AND manufacturer != ''
  AND model IS NOT NULL
  AND model != ''
  AND year > 0
  AND price > 5000
GROUP BY manufacturer, model, year
HAVING COUNT(*) >= 10;

CREATE UNIQUE INDEX IF NOT EXISTS idx_market_medians_lookup
    ON market_medians (manufacturer, model, year);
