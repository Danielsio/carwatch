package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// countSearchListingsForChatSQL batches per-search listing counts using each search row's caps.
const countSearchListingsForChatSQL = `
SELECT lh.search_id, COUNT(*)
FROM listing_history lh
INNER JOIN searches s ON s.id = lh.search_id AND s.chat_id = lh.chat_id
WHERE lh.chat_id = $1
  AND (COALESCE(s.price_min, 0) <= 0 OR lh.price >= s.price_min OR lh.price = 0)
  AND (s.price_max <= 0 OR lh.price <= s.price_max)
  AND (s.year_min <= 0 OR lh.year >= s.year_min)
  AND (s.year_max <= 0 OR lh.year <= s.year_max)
  AND (s.max_km <= 0 OR lh.km <= s.max_km OR lh.km = 0)
  AND (s.max_hand <= 0 OR lh.hand <= s.max_hand)
  AND CASE LOWER(TRIM(COALESCE(NULLIF(s.seller_filter, ''), 'any')))
    WHEN 'private' THEN lh.is_commercial = 0
    WHEN 'commercial' THEN lh.is_commercial = 1
    WHEN 'dealer' THEN lh.is_commercial = 1
    WHEN 'dealership' THEN lh.is_commercial = 1
    ELSE TRUE
  END
  AND (COALESCE(s.gear_box, '') = '' OR LOWER(lh.gear_box) = LOWER(s.gear_box))
  AND (NOT s.price_only OR lh.price > 0)
  AND (NOT s.photo_only OR lh.image_url != '')
GROUP BY lh.search_id`

const upsertListingSQL = `
	INSERT INTO listing_history
	(token, chat_id, search_id, search_name, manufacturer, model, sub_model, sub_model_id, year, price, km, hand, city, page_link, image_url, engine_volume, horse_power, engine_type, gear_box, description, is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27)
	ON CONFLICT (token, chat_id) DO UPDATE SET
		search_id = CASE WHEN EXCLUDED.search_id > 0 THEN EXCLUDED.search_id ELSE listing_history.search_id END,
		search_name = CASE WHEN EXCLUDED.search_id > 0 THEN EXCLUDED.search_name ELSE listing_history.search_name END,
		manufacturer = CASE WHEN EXCLUDED.manufacturer != '' THEN EXCLUDED.manufacturer ELSE listing_history.manufacturer END,
		model = CASE WHEN EXCLUDED.model != '' THEN EXCLUDED.model ELSE listing_history.model END,
		sub_model = CASE WHEN EXCLUDED.sub_model != '' THEN EXCLUDED.sub_model ELSE listing_history.sub_model END,
		sub_model_id = CASE WHEN EXCLUDED.sub_model_id > 0 THEN EXCLUDED.sub_model_id ELSE listing_history.sub_model_id END,
		year = CASE WHEN EXCLUDED.year > 0 THEN EXCLUDED.year ELSE listing_history.year END,
		price = EXCLUDED.price,
		km = CASE WHEN EXCLUDED.km > 0 THEN EXCLUDED.km ELSE listing_history.km END,
		hand = EXCLUDED.hand,
		city = CASE WHEN EXCLUDED.city != '' THEN EXCLUDED.city ELSE listing_history.city END,
		page_link = CASE WHEN EXCLUDED.page_link != '' THEN EXCLUDED.page_link ELSE listing_history.page_link END,
		image_url = CASE WHEN EXCLUDED.image_url != '' THEN EXCLUDED.image_url ELSE listing_history.image_url END,
		engine_volume = CASE WHEN EXCLUDED.engine_volume > 0 THEN EXCLUDED.engine_volume ELSE listing_history.engine_volume END,
		horse_power = CASE WHEN EXCLUDED.horse_power > 0 THEN EXCLUDED.horse_power ELSE listing_history.horse_power END,
		engine_type = CASE WHEN EXCLUDED.engine_type != '' THEN EXCLUDED.engine_type ELSE listing_history.engine_type END,
		gear_box = CASE WHEN EXCLUDED.gear_box != '' THEN EXCLUDED.gear_box ELSE listing_history.gear_box END,
		description = CASE WHEN EXCLUDED.description != '' THEN EXCLUDED.description ELSE listing_history.description END,
		is_commercial = COALESCE(EXCLUDED.is_commercial, listing_history.is_commercial),
		fitness_score = COALESCE(EXCLUDED.fitness_score, listing_history.fitness_score),
		median_price = COALESCE(EXCLUDED.median_price, listing_history.median_price),
		cohort_size = COALESCE(EXCLUDED.cohort_size, listing_history.cohort_size),
		deal_score = COALESCE(EXCLUDED.deal_score, listing_history.deal_score),
		base_price = COALESCE(EXCLUDED.base_price, listing_history.base_price),
		removed_at = NULL`

type listingScanner interface {
	Scan(dest ...any) error
}

func scanListingRow(sc listingScanner) (storage.ListingRecord, error) {
	var l storage.ListingRecord
	var fs sql.NullFloat64
	var ic, mp, cs, ds, bp sql.NullInt64
	var removedAt sql.NullTime
	if err := sc.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model, &l.SubModel, &l.SubModelID,
		&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL,
		&l.EngineVolume, &l.HorsePower, &l.EngineType, &l.GearBox, &l.Description,
		&ic, &fs, &mp, &cs, &ds, &bp, &l.FirstSeenAt, &removedAt); err != nil {
		return l, err
	}
	l.IsCommercial = storage.ListingCommercialFromSQL(ic)
	if fs.Valid {
		l.FitnessScore = &fs.Float64
	}
	if mp.Valid {
		v := int(mp.Int64)
		l.MedianPrice = &v
	}
	if cs.Valid {
		v := int(cs.Int64)
		l.CohortSize = &v
	}
	if ds.Valid {
		v := int(ds.Int64)
		l.DealScore = &v
	}
	if bp.Valid {
		v := int(bp.Int64)
		l.BasePrice = &v
	}
	if removedAt.Valid {
		l.RemovedAt = &removedAt.Time
	}
	return l, nil
}

func upsertListingArgs(r storage.ListingRecord) []any {
	return []any{
		r.Token, r.ChatID, r.SearchID, r.SearchName, r.Manufacturer, r.Model, r.SubModel, r.SubModelID, r.Year, r.Price,
		r.Km, r.Hand, r.City, r.PageLink, r.ImageURL,
		r.EngineVolume, r.HorsePower, r.EngineType, r.GearBox, r.Description,
		storage.ListingCommercialToSQL(r.IsCommercial),
		r.FitnessScore, r.MedianPrice, r.CohortSize, r.DealScore, r.BasePrice, r.FirstSeenAt.UTC(),
	}
}

func (s *Store) SaveListing(ctx context.Context, r storage.ListingRecord) error {
	_, err := s.db.ExecContext(ctx, upsertListingSQL, upsertListingArgs(r)...)
	if err != nil {
		return fmt.Errorf("save listing: %w", err)
	}
	return nil
}

func (s *Store) SaveListings(ctx context.Context, records []storage.ListingRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save listings begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, upsertListingSQL)
	if err != nil {
		return fmt.Errorf("save listings prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, upsertListingArgs(r)...); err != nil {
			return fmt.Errorf("save listings exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save listings commit: %w", err)
	}
	return nil
}

const backfillListingSQL = `
	UPDATE listing_history SET
		km = CASE WHEN $2 > 0 AND listing_history.km <= 0 THEN $2 ELSE listing_history.km END,
		city = CASE WHEN $3 != '' AND listing_history.city = '' THEN $3 ELSE listing_history.city END,
		image_url = CASE WHEN $4 != '' AND listing_history.image_url = '' THEN $4 ELSE listing_history.image_url END
	WHERE token = $1 AND (listing_history.km <= 0 OR listing_history.city = '' OR listing_history.image_url = '')`

func (s *Store) BackfillListings(ctx context.Context, records []storage.ListingRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("backfill begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, backfillListingSQL)
	if err != nil {
		return fmt.Errorf("backfill prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.Token, r.Km, r.City, r.ImageURL); err != nil {
			return fmt.Errorf("backfill exec: %w", err)
		}
	}
	return tx.Commit()
}

const lookupEnrichmentBatchSize = 500

func (s *Store) LookupEnrichmentData(ctx context.Context, tokens []string) (map[string]storage.EnrichmentRecord, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	out := make(map[string]storage.EnrichmentRecord, len(tokens))
	for start := 0; start < len(tokens); start += lookupEnrichmentBatchSize {
		end := start + lookupEnrichmentBatchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		batch := tokens[start:end]

		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, t := range batch {
			args[i] = t
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		q := `SELECT DISTINCT ON (token) token, km, city, image_url
			FROM listing_history
			WHERE token IN (` + strings.Join(placeholders, ", ") + `) AND (km > 0 OR city != '' OR image_url != '')
			ORDER BY token, first_seen_at DESC`

		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("lookup enrichment data: %w", err)
		}

		for rows.Next() {
			var tok, city, img string
			var km int
			if err := rows.Scan(&tok, &km, &city, &img); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan enrichment data: %w", err)
			}
			out[tok] = storage.EnrichmentRecord{Km: km, City: city, ImageURL: img}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("enrichment rows: %w", err)
		}
		_ = rows.Close()
	}
	return out, nil
}

func (s *Store) GetListing(ctx context.Context, chatID int64, token string) (*storage.ListingRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, removed_at
		FROM listing_history
		WHERE chat_id = $1 AND token = $2
		ORDER BY first_seen_at DESC
		LIMIT 1`, chatID, token)
	l, err := scanListingRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (s *Store) ListUserListings(ctx context.Context, chatID int64, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, removed_at
		FROM listing_history
		WHERE chat_id = $1
		ORDER BY first_seen_at DESC, token DESC
		LIMIT $2 OFFSET $3`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		l, err := scanListingRow(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) CountUserListings(ctx context.Context, chatID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM listing_history
		WHERE chat_id = $1`, chatID).Scan(&count)
	return count, err
}

func (s *Store) ListListings(ctx context.Context, limit int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, year, price, km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, removed_at
		FROM (
			SELECT DISTINCT ON (token)
				token, search_name, manufacturer, model, sub_model, sub_model_id, year, price, km, hand, city, page_link, image_url,
				engine_volume, horse_power, engine_type, gear_box, description,
				is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, removed_at
			FROM listing_history
			ORDER BY token, first_seen_at DESC
		) deduped
		ORDER BY first_seen_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		l, err := scanListingRow(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func buildFilterClauses(f storage.ListingFilter, paramStart int) (string, []any, int) {
	var clauses []string
	var args []any
	n := paramStart
	if f.PriceMin > 0 {
		clauses = append(clauses, fmt.Sprintf("(price >= $%d OR price = 0)", n))
		args = append(args, f.PriceMin)
		n++
	}
	if f.PriceMax > 0 {
		clauses = append(clauses, fmt.Sprintf("price <= $%d", n))
		args = append(args, f.PriceMax)
		n++
	}
	if f.YearMin > 0 {
		clauses = append(clauses, fmt.Sprintf("year >= $%d", n))
		args = append(args, f.YearMin)
		n++
	}
	if f.YearMax > 0 {
		clauses = append(clauses, fmt.Sprintf("year <= $%d", n))
		args = append(args, f.YearMax)
		n++
	}
	if f.MaxKm > 0 {
		clauses = append(clauses, fmt.Sprintf("(km <= $%d OR km = 0)", n))
		args = append(args, f.MaxKm)
		n++
	}
	if f.MaxHand > 0 {
		clauses = append(clauses, fmt.Sprintf("hand <= $%d", n))
		args = append(args, f.MaxHand)
		n++
	}
	if f.Commercial != nil {
		if *f.Commercial {
			clauses = append(clauses, fmt.Sprintf("is_commercial = $%d", n))
			args = append(args, 1)
		} else {
			clauses = append(clauses, fmt.Sprintf("is_commercial = $%d", n))
			args = append(args, 0)
		}
		n++
	}
	if f.GearBox != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(gear_box) = LOWER($%d)", n))
		args = append(args, f.GearBox)
		n++
	}
	if f.PriceOnly {
		clauses = append(clauses, "price > 0")
	}
	if f.PhotoOnly {
		clauses = append(clauses, "image_url != ''")
	}
	if len(clauses) == 0 {
		return "", nil, paramStart
	}
	return " AND " + strings.Join(clauses, " AND "), args, n
}

func (s *Store) ListSearchListings(ctx context.Context, chatID int64, searchID int64, f storage.ListingFilter, limit, offset int, sort string) ([]storage.ListingRecord, error) {
	orderBy := "first_seen_at DESC, token DESC"
	switch sort {
	case "newest":
		orderBy = "first_seen_at DESC, token DESC"
	case "price_asc":
		orderBy = "CASE WHEN price <= 0 THEN 1 ELSE 0 END, price ASC, token DESC"
	case "price_desc":
		orderBy = "CASE WHEN price <= 0 THEN 1 ELSE 0 END, price DESC, token DESC"
	case "score":
		orderBy = "CASE WHEN fitness_score IS NULL THEN 1 ELSE 0 END, fitness_score DESC, token DESC"
	case "km":
		orderBy = "CASE WHEN km <= 0 THEN 1 ELSE 0 END, km ASC, token DESC"
	case "year":
		orderBy = "year DESC, token DESC"
	}

	filterSQL, filterArgs, nextParam := buildFilterClauses(f, 3)
	args := []any{chatID, searchID}
	args = append(args, filterArgs...)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, removed_at
		FROM listing_history
		WHERE chat_id = $1 AND search_id = $2%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, filterSQL, orderBy, nextParam, nextParam+1), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		l, err := scanListingRow(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) CountSearchListings(ctx context.Context, chatID int64, searchID int64, f storage.ListingFilter) (int64, error) {
	filterSQL, filterArgs, _ := buildFilterClauses(f, 3)
	args := []any{chatID, searchID}
	args = append(args, filterArgs...)

	var count int64
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM listing_history
		WHERE chat_id = $1 AND search_id = $2%s`, filterSQL), args...).Scan(&count)
	return count, err
}

func (s *Store) CountSearchListingsForChat(ctx context.Context, chatID int64) (map[int64]int64, error) {
	rows, err := s.db.QueryContext(ctx, countSearchListingsForChatSQL, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64]int64)
	for rows.Next() {
		var searchID int64
		var n int64
		if err := rows.Scan(&searchID, &n); err != nil {
			return nil, err
		}
		out[searchID] = n
	}
	return out, rows.Err()
}

func (s *Store) SearchStats(ctx context.Context, chatID int64, searchID int64, f storage.ListingFilter) (*storage.SearchStats, error) {
	filterSQL, filterArgs, _ := buildFilterClauses(f, 3)

	query := `
		SELECT
			COUNT(*) AS total,
			COUNT(CASE WHEN first_seen_at > NOW() - INTERVAL '1 day' THEN 1 END) AS new_24h,
			AVG(CASE WHEN price > 0 THEN price END) AS avg_price,
			MIN(CASE WHEN price > 0 THEN price END) AS min_price,
			MAX(CASE WHEN price > 0 THEN price END) AS max_price
		FROM listing_history
		WHERE search_id = $1 AND chat_id = $2` + filterSQL

	args := append([]any{searchID, chatID}, filterArgs...)

	var st storage.SearchStats
	var avgPrice, minPrice, maxPrice sql.NullFloat64
	err := s.db.QueryRowContext(ctx, query, args...).
		Scan(&st.Total, &st.New24h, &avgPrice, &minPrice, &maxPrice)
	if err != nil {
		return nil, fmt.Errorf("search stats: %w", err)
	}
	if avgPrice.Valid {
		st.AvgPrice = avgPrice.Float64
	}
	if minPrice.Valid {
		st.MinPrice = int(minPrice.Float64)
	}
	if maxPrice.Valid {
		st.MaxPrice = int(maxPrice.Float64)
	}
	return &st, nil
}

func (s *Store) DeleteStaleListings(ctx context.Context, chatID int64, searchID int64, keepTokens []string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("delete stale begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var staleFilter string
	args := []interface{}{chatID, searchID}
	if len(keepTokens) > 0 {
		placeholders := make([]string, len(keepTokens))
		for i, t := range keepTokens {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, t)
		}
		staleFilter = fmt.Sprintf(" AND token NOT IN (%s)", strings.Join(placeholders, ","))
	}

	// Mark bookmarked stale listings as removed_at (likely sold).
	markArgs := make([]interface{}, len(args))
	copy(markArgs, args)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE listing_history
		SET removed_at = NOW()
		WHERE chat_id = $1 AND search_id = $2%s
		  AND removed_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM saved_listings
		    WHERE saved_listings.token = listing_history.token
		      AND saved_listings.chat_id = listing_history.chat_id
		  )`, staleFilter), markArgs...)
	if err != nil {
		return 0, fmt.Errorf("mark removed_at: %w", err)
	}

	// Delete non-bookmarked stale listings.
	deleteArgs := make([]interface{}, len(args))
	copy(deleteArgs, args)
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM listing_history
		WHERE chat_id = $1 AND search_id = $2%s
		  AND NOT EXISTS (
		    SELECT 1 FROM saved_listings
		    WHERE saved_listings.token = listing_history.token
		      AND saved_listings.chat_id = listing_history.chat_id
		  )`, staleFilter), deleteArgs...)
	if err != nil {
		return 0, fmt.Errorf("delete stale: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("delete stale commit: %w", err)
	}
	return removed, nil
}

func (s *Store) ListUnenrichedTokens(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT token FROM (
			SELECT DISTINCT ON (token) token, first_seen_at FROM listing_history
			WHERE km <= 0
			  AND COALESCE(city, '') = ''
			  AND COALESCE(image_url, '') = ''
			  AND enrich_attempts < 10
			  AND (last_enrich_at IS NULL OR last_enrich_at < NOW() - INTERVAL '1 hour')
			ORDER BY token, first_seen_at DESC
		) t
		ORDER BY first_seen_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *Store) CountUnenrichedTokens(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT token)
		 FROM listing_history
		 WHERE km <= 0
		   AND COALESCE(city, '') = ''
		   AND COALESCE(image_url, '') = ''
		   AND enrich_attempts < 10`).Scan(&count)
	return count, err
}

func (s *Store) IncrementEnrichAttempt(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE listing_history SET enrich_attempts = enrich_attempts + 1, last_enrich_at = NOW() WHERE token = $1`,
		token)
	return err
}

func (s *Store) PruneListings(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM listing_history
		WHERE first_seen_at < $1
		  AND NOT EXISTS (
		      SELECT 1 FROM saved_listings
		      WHERE saved_listings.token = listing_history.token
		        AND saved_listings.chat_id = listing_history.chat_id
		  )`,
		cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) SaveBookmark(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO saved_listings (chat_id, token)
		VALUES ($1, $2)
		ON CONFLICT (chat_id, token) DO NOTHING`,
		chatID, token)
	return err
}

func (s *Store) RemoveBookmark(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM saved_listings WHERE chat_id = $1 AND token = $2`,
		chatID, token)
	return err
}

func (s *Store) ListSaved(ctx context.Context, chatID int64, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT lh.token, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.year, lh.price,
			lh.km, lh.hand, lh.city, lh.page_link, lh.image_url,
			lh.engine_volume, lh.horse_power, lh.engine_type, lh.gear_box, lh.description,
			lh.is_commercial, lh.fitness_score, lh.median_price, lh.cohort_size, lh.deal_score, lh.base_price, lh.first_seen_at, lh.removed_at
		FROM saved_listings sl
		JOIN listing_history lh ON sl.token = lh.token AND sl.chat_id = lh.chat_id
		WHERE sl.chat_id = $1
		ORDER BY sl.saved_at DESC
		LIMIT $2 OFFSET $3`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		l, err := scanListingRow(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) CountSaved(ctx context.Context, chatID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM saved_listings WHERE chat_id = $1`, chatID).Scan(&count)
	return count, err
}

func (s *Store) IsSaved(ctx context.Context, chatID int64, token string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM saved_listings WHERE chat_id = $1 AND token = $2`,
		chatID, token).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

const savedAmongBatchSize = 500

func (s *Store) SavedAmong(ctx context.Context, chatID int64, tokens []string) (map[string]bool, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	uniq := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		uniq = append(uniq, t)
	}
	if len(uniq) == 0 {
		return nil, nil
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin read tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	out := make(map[string]bool)
	for start := 0; start < len(uniq); start += savedAmongBatchSize {
		end := start + savedAmongBatchSize
		if end > len(uniq) {
			end = len(uniq)
		}
		batch := uniq[start:end]

		placeholders := make([]string, len(batch))
		args := make([]any, 0, 1+len(batch))
		args = append(args, chatID)
		for i := range batch {
			args = append(args, batch[i])
			placeholders[i] = fmt.Sprintf("$%d", 2+i)
		}

		q := `SELECT token FROM saved_listings WHERE chat_id = $1 AND token IN (` + strings.Join(placeholders, ", ") + `)`
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var tok string
			if err := rows.Scan(&tok); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[tok] = true
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, tx.Commit()
}

func (s *Store) HideListing(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO hidden_listings (chat_id, token)
		VALUES ($1, $2)
		ON CONFLICT (chat_id, token) DO NOTHING`,
		chatID, token)
	return err
}

func (s *Store) UnhideListing(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM hidden_listings WHERE chat_id = $1 AND token = $2`,
		chatID, token)
	return err
}

func (s *Store) IsHidden(ctx context.Context, chatID int64, token string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM hidden_listings WHERE chat_id = $1 AND token = $2`,
		chatID, token).Scan(&count)
	return count > 0, err
}

func (s *Store) ListHidden(ctx context.Context, chatID int64, limit, offset int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT token FROM hidden_listings WHERE chat_id = $1 ORDER BY hidden_at DESC LIMIT $2 OFFSET $3`,
		chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *Store) CountHidden(ctx context.Context, chatID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM hidden_listings WHERE chat_id = $1`, chatID).Scan(&count)
	return count, err
}

func (s *Store) ClearHidden(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM hidden_listings WHERE chat_id = $1`, chatID)
	return err
}

func (s *Store) ListHiddenTokens(ctx context.Context, chatID int64) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT token FROM hidden_listings WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tokens := make(map[string]bool)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens[t] = true
	}
	return tokens, rows.Err()
}
