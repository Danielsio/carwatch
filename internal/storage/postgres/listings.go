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
    WHEN 'private' THEN COALESCE(lh.is_commercial, 0) = 0
    WHEN 'commercial' THEN COALESCE(lh.is_commercial, 0) = 1
    WHEN 'dealer' THEN COALESCE(lh.is_commercial, 0) = 1
    WHEN 'dealership' THEN COALESCE(lh.is_commercial, 0) = 1
    ELSE TRUE
  END
  AND (COALESCE(s.gear_box, '') = '' OR lh.gear_box = '' OR LOWER(lh.gear_box) = LOWER(s.gear_box))
  AND (NOT s.price_only OR lh.price > 0)
  AND (NOT s.photo_only OR lh.image_url != '')
  AND lh.removed_at IS NULL
GROUP BY lh.search_id`

const upsertListingSQL = `
	INSERT INTO listing_history
	(token, chat_id, search_id, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price, km, hand, city, page_link, image_url, engine_volume, horse_power, engine_type, gear_box, description, is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, posted_at, score_condition, score_value, score_engine, original_ownership)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33)
	ON CONFLICT (token, chat_id) DO UPDATE SET
		search_id = CASE WHEN EXCLUDED.search_id > 0 THEN EXCLUDED.search_id ELSE listing_history.search_id END,
		search_name = CASE WHEN EXCLUDED.search_id > 0 THEN EXCLUDED.search_name ELSE listing_history.search_name END,
		manufacturer = CASE WHEN EXCLUDED.manufacturer != '' THEN EXCLUDED.manufacturer ELSE listing_history.manufacturer END,
		model = CASE WHEN EXCLUDED.model != '' THEN EXCLUDED.model ELSE listing_history.model END,
		sub_model = CASE WHEN EXCLUDED.sub_model != '' THEN EXCLUDED.sub_model ELSE listing_history.sub_model END,
		sub_model_id = CASE WHEN EXCLUDED.sub_model_id > 0 THEN EXCLUDED.sub_model_id ELSE listing_history.sub_model_id END,
		body_type = CASE WHEN EXCLUDED.body_type != '' THEN EXCLUDED.body_type ELSE listing_history.body_type END,
		year = CASE WHEN EXCLUDED.year > 0 THEN EXCLUDED.year ELSE listing_history.year END,
		-- A price CHANGE is the point of the product, so real values always win
		-- — but 0 is not a price. It is what the parser yields when the feed row
		-- omits the field, and letting it through erased the known price of a
		-- listing (breaking the price_only filter, price sorting, deal scores and
		-- the price-drop signal itself) on any partial payload. Same for hand,
		-- which only ever counts upward: a 0 in an update is a missing field, not
		-- a car that lost an owner. Zeros still insert fine on first sight.
		price = CASE WHEN EXCLUDED.price > 0 THEN EXCLUDED.price ELSE listing_history.price END,
		km = CASE WHEN EXCLUDED.km > 0 THEN EXCLUDED.km ELSE listing_history.km END,
		hand = CASE WHEN EXCLUDED.hand > 0 THEN EXCLUDED.hand ELSE listing_history.hand END,
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
		posted_at = COALESCE(EXCLUDED.posted_at, listing_history.posted_at),
		removed_at = NULL,
		-- Re-observed: renew the retention clock (see PruneListings).
		last_seen_at = NOW(),
		score_condition = COALESCE(EXCLUDED.score_condition, listing_history.score_condition),
		score_value = COALESCE(EXCLUDED.score_value, listing_history.score_value),
		score_engine = COALESCE(EXCLUDED.score_engine, listing_history.score_engine),
		original_ownership = COALESCE(EXCLUDED.original_ownership, listing_history.original_ownership)`

// Note: body_type is intentionally in the backlog selection but NOT in
// unenrichedMissingFieldsWhereSQL (which drives ResetUnenrichedAttempts). This
// lets the enricher re-fetch listings missing only a body type, while the
// 10-attempt cap still bounds listings whose item page never yields one
// (otherwise the reset query would keep clearing their attempts → re-fetch loop).
const unenrichedBacklogWhereSQL = `
(km <= 0 OR COALESCE(city, '') = '' OR COALESCE(image_url, '') = '' OR COALESCE(original_ownership, '') = '' OR COALESCE(body_type, '') = '')
AND enrich_attempts < 10
AND (last_enrich_at IS NULL OR last_enrich_at < NOW() - INTERVAL '1 hour')
AND first_seen_at > NOW() - INTERVAL '7 days'`

const unenrichedMissingFieldsWhereSQL = `
(km <= 0 OR COALESCE(city, '') = '' OR COALESCE(image_url, '') = '' OR COALESCE(original_ownership, '') = '')`

type listingScanner interface {
	Scan(dest ...any) error
}

func scanListingRow(sc listingScanner) (storage.ListingRecord, error) {
	var l storage.ListingRecord
	var fs, scoreCond, scoreVal, scoreEng sql.NullFloat64
	var ic, mp, cs, ds, bp sql.NullInt64
	var removedAt, postedAt sql.NullTime
	var origOwn sql.NullString
	if err := sc.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model, &l.SubModel, &l.SubModelID, &l.BodyType,
		&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL,
		&l.EngineVolume, &l.HorsePower, &l.EngineType, &l.GearBox, &l.Description,
		&ic, &fs, &mp, &cs, &ds, &bp, &l.FirstSeenAt, &postedAt, &removedAt,
		&scoreCond, &scoreVal, &scoreEng, &origOwn); err != nil {
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
	if postedAt.Valid {
		l.PostedAt = &postedAt.Time
	}
	if removedAt.Valid {
		l.RemovedAt = &removedAt.Time
	}
	if scoreCond.Valid {
		v := scoreCond.Float64
		l.ScoreCondition = &v
	}
	if scoreVal.Valid {
		v := scoreVal.Float64
		l.ScoreValue = &v
	}
	if scoreEng.Valid {
		v := scoreEng.Float64
		l.ScoreEngine = &v
	}
	if origOwn.Valid && origOwn.String != "" {
		s := origOwn.String
		l.OriginalOwnership = &s
	}
	return l, nil
}

func upsertListingArgs(r storage.ListingRecord) []any {
	return []any{
		r.Token, r.ChatID, r.SearchID, r.SearchName, r.Manufacturer, r.Model, r.SubModel, r.SubModelID, r.BodyType, r.Year, r.Price,
		r.Km, r.Hand, r.City, r.PageLink, r.ImageURL,
		r.EngineVolume, r.HorsePower, r.EngineType, r.GearBox, r.Description,
		storage.ListingCommercialToSQL(r.IsCommercial),
		r.FitnessScore, r.MedianPrice, r.CohortSize, r.DealScore, r.BasePrice, r.FirstSeenAt.UTC(), r.PostedAt,
		r.ScoreCondition, r.ScoreValue, r.ScoreEngine, r.OriginalOwnership,
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
		image_url = CASE WHEN $4 != '' AND listing_history.image_url = '' THEN $4 ELSE listing_history.image_url END,
		body_type = CASE WHEN $5 != '' AND listing_history.body_type = '' THEN $5 ELSE listing_history.body_type END
	WHERE token = $1 AND (listing_history.km <= 0 OR listing_history.city = '' OR listing_history.image_url = '' OR listing_history.body_type = '')`

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
		if _, err := stmt.ExecContext(ctx, r.Token, r.Km, r.City, r.ImageURL, r.BodyType); err != nil {
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

		q := `SELECT DISTINCT ON (token) token, manufacturer, model, year, price, search_name, km, city, image_url, COALESCE(body_type, '')
			FROM listing_history
			WHERE token IN (` + strings.Join(placeholders, ", ") + `) AND (km > 0 OR city != '' OR image_url != '')
			ORDER BY token, first_seen_at DESC`

		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("lookup enrichment data: %w", err)
		}

		for rows.Next() {
			var tok, manufacturer, model, searchName, city, img, bodyType string
			var year, price, km int
			if err := rows.Scan(&tok, &manufacturer, &model, &year, &price, &searchName, &km, &city, &img, &bodyType); err != nil {
				return nil, errors.Join(fmt.Errorf("scan enrichment data: %w", err), rows.Close())
			}
			out[tok] = storage.EnrichmentRecord{
				Manufacturer: manufacturer, Model: model, Year: year, Price: price, SearchName: searchName,
				Km: km, City: city, ImageURL: img, BodyType: bodyType,
			}
		}
		if err := rows.Err(); err != nil {
			return nil, errors.Join(fmt.Errorf("enrichment rows: %w", err), rows.Close())
		}
		_ = rows.Close()
	}
	return out, nil
}

func (s *Store) LookupListingIdentity(ctx context.Context, token string) (*storage.ListingIdentity, error) {
	var id storage.ListingIdentity
	err := s.db.QueryRowContext(ctx,
		`SELECT manufacturer, model, year, price, search_name
		 FROM listing_history WHERE token = $1
		 ORDER BY first_seen_at DESC LIMIT 1`, token).
		Scan(&id.Manufacturer, &id.Model, &id.Year, &id.Price, &id.SearchName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("lookup listing identity: %w", err)
	}
	return &id, nil
}

func (s *Store) GetListing(ctx context.Context, chatID int64, token string) (*storage.ListingRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, posted_at, removed_at,
			score_condition, score_value, score_engine, original_ownership
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
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, posted_at, removed_at,
			score_condition, score_value, score_engine, original_ownership
		FROM listing_history
		WHERE chat_id = $1 AND removed_at IS NULL
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
		WHERE chat_id = $1 AND removed_at IS NULL`, chatID).Scan(&count)
	return count, err
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
			clauses = append(clauses, fmt.Sprintf("COALESCE(is_commercial, 0) = $%d", n))
			args = append(args, 1)
		} else {
			clauses = append(clauses, fmt.Sprintf("COALESCE(is_commercial, 0) = $%d", n))
			args = append(args, 0)
		}
		n++
	}
	if f.GearBox != "" {
		clauses = append(clauses, fmt.Sprintf("(gear_box = '' OR LOWER(gear_box) = LOWER($%d))", n))
		args = append(args, f.GearBox)
		n++
	}
	if f.PriceOnly {
		clauses = append(clauses, "price > 0")
	}
	if f.PhotoOnly {
		clauses = append(clauses, "image_url != ''")
	}
	if !f.IncludeRemoved {
		clauses = append(clauses, "removed_at IS NULL")
	}
	if len(clauses) == 0 {
		return "", nil, paramStart
	}
	return " AND " + strings.Join(clauses, " AND "), args, n
}

// newestOrderBy sorts by the listing's original post date on the source (yad2),
// falling back to first_seen_at when posted_at was never captured.
const newestOrderBy = "COALESCE(posted_at, first_seen_at) DESC, token DESC"

// listingsOrderBy maps a user-facing sort key to a SQL ORDER BY clause. An
// empty or unknown sort defaults to newest.
func listingsOrderBy(sort string) string {
	switch sort {
	case "price_asc":
		return "CASE WHEN price <= 0 THEN 1 ELSE 0 END, price ASC, token DESC"
	case "price_desc":
		return "CASE WHEN price <= 0 THEN 1 ELSE 0 END, price DESC, token DESC"
	case "score":
		return "CASE WHEN fitness_score IS NULL THEN 1 ELSE 0 END, fitness_score DESC, token DESC"
	case "km":
		return "CASE WHEN km <= 0 THEN 1 ELSE 0 END, km ASC, token DESC"
	case "year":
		return "year DESC, token DESC"
	default: // "newest" and unknown values
		return newestOrderBy
	}
}

func (s *Store) ListSearchListings(ctx context.Context, chatID int64, searchID int64, f storage.ListingFilter, limit, offset int, sort string) ([]storage.ListingRecord, error) {
	orderBy := listingsOrderBy(sort)

	filterSQL, filterArgs, nextParam := buildFilterClauses(f, 3)
	args := []any{chatID, searchID}
	args = append(args, filterArgs...)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT token, search_name, manufacturer, model, sub_model, sub_model_id, body_type, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at, posted_at, removed_at,
			score_condition, score_value, score_engine, original_ownership
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

// DropListingByToken removes a listing whose source page no longer exists
// (HTTP 404/410). It mirrors DeleteStaleListings but is keyed by token alone,
// so it removes the listing for every chat/search that holds it: bookmarked
// copies are flagged removed_at ("likely sold") and kept, all other copies are
// hard-deleted. Returns the number of rows hard-deleted.
func (s *Store) DropListingByToken(ctx context.Context, token string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("drop listing begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Preserve bookmarked copies as "likely sold".
	if _, err := tx.ExecContext(ctx, `
		UPDATE listing_history
		SET removed_at = NOW()
		WHERE token = $1
		  AND removed_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM saved_listings
		    WHERE saved_listings.token = listing_history.token
		      AND saved_listings.chat_id = listing_history.chat_id
		  )`, token); err != nil {
		return 0, fmt.Errorf("drop listing mark removed_at: %w", err)
	}

	// Hard-delete every non-bookmarked copy.
	result, err := tx.ExecContext(ctx, `
		DELETE FROM listing_history
		WHERE token = $1
		  AND NOT EXISTS (
		    SELECT 1 FROM saved_listings
		    WHERE saved_listings.token = listing_history.token
		      AND saved_listings.chat_id = listing_history.chat_id
		  )`, token)
	if err != nil {
		return 0, fmt.Errorf("drop listing delete: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("drop listing commit: %w", err)
	}
	return removed, nil
}

// MarkListingsRemoved retires listings the source no longer serves, for ONE
// chat. Bookmarked copies keep their row with removed_at set ("likely sold")
// so the user's saved car does not vanish; every other copy is hard-deleted.
// Returns the number of rows hard-deleted.
//
// Scoped to chat_id on purpose: the tokens come from an extension ingest push,
// so a buggy or hostile client must only ever be able to retire its own
// listings, never another user's. (DropListingByToken, which is global, is an
// admin operation.)
func (s *Store) MarkListingsRemoved(ctx context.Context, chatID int64, tokens []string) (int64, error) {
	if len(tokens) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("mark removed begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(tokens))
	args := []interface{}{chatID}
	for i, t := range tokens {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, t)
	}
	tokenFilter := fmt.Sprintf(" AND token IN (%s)", strings.Join(placeholders, ","))

	markArgs := make([]interface{}, len(args))
	copy(markArgs, args)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE listing_history
		SET removed_at = NOW()
		WHERE chat_id = $1%s
		  AND removed_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM saved_listings
		    WHERE saved_listings.token = listing_history.token
		      AND saved_listings.chat_id = listing_history.chat_id
		  )`, tokenFilter), markArgs...); err != nil {
		return 0, fmt.Errorf("mark removed_at: %w", err)
	}

	deleteArgs := make([]interface{}, len(args))
	copy(deleteArgs, args)
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM listing_history
		WHERE chat_id = $1%s
		  AND NOT EXISTS (
		    SELECT 1 FROM saved_listings
		    WHERE saved_listings.token = listing_history.token
		      AND saved_listings.chat_id = listing_history.chat_id
		  )`, tokenFilter), deleteArgs...)
	if err != nil {
		return 0, fmt.Errorf("delete removed: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("mark removed commit: %w", err)
	}
	return deleted, nil
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
			SELECT DISTINCT ON (lh.token) lh.token, lh.first_seen_at
			FROM listing_history lh
			JOIN searches s ON s.id = lh.search_id AND s.chat_id = lh.chat_id AND s.active = true
			WHERE `+unenrichedBacklogWhereSQL+`
			  AND (s.price_max = 0 OR lh.price <= s.price_max)
			  AND (s.price_min = 0 OR lh.price >= s.price_min)
			  AND (s.year_min = 0 OR lh.year >= s.year_min)
			  AND (s.year_max = 0 OR lh.year <= s.year_max)
			ORDER BY lh.token, lh.first_seen_at DESC
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
		`SELECT COUNT(*)
		 FROM (
		   SELECT DISTINCT ON (lh.token) lh.token
		   FROM listing_history lh
		   JOIN searches s ON s.id = lh.search_id AND s.chat_id = lh.chat_id AND s.active = true
		   WHERE `+unenrichedBacklogWhereSQL+`
		     AND (s.price_max = 0 OR lh.price <= s.price_max)
		     AND (s.price_min = 0 OR lh.price >= s.price_min)
		     AND (s.year_min = 0 OR lh.year >= s.year_min)
		     AND (s.year_max = 0 OR lh.year <= s.year_max)
		   ORDER BY lh.token, lh.first_seen_at DESC
		 ) t`).Scan(&count)
	return count, err
}

func (s *Store) CountUnenrichedCooldownTokens(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM (
		   SELECT DISTINCT ON (token) token, enrich_attempts, last_enrich_at
		   FROM listing_history
		   WHERE `+unenrichedMissingFieldsWhereSQL+`
		   ORDER BY token, first_seen_at DESC
		 ) t
		 WHERE enrich_attempts < 10
		   AND last_enrich_at IS NOT NULL
		   AND last_enrich_at >= NOW() - INTERVAL '1 hour'`).Scan(&count)
	return count, err
}

func (s *Store) CountUnenrichedExhaustedTokens(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM (
		   SELECT DISTINCT ON (token) token, enrich_attempts
		   FROM listing_history
		   WHERE `+unenrichedMissingFieldsWhereSQL+`
		   ORDER BY token, first_seen_at DESC
		 ) t
		 WHERE enrich_attempts >= 10`).Scan(&count)
	return count, err
}

func (s *Store) IncrementEnrichAttempt(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE listing_history SET enrich_attempts = enrich_attempts + 1, last_enrich_at = NOW() WHERE token = $1`,
		token)
	return err
}

func (s *Store) ListEnrichExhaustedTokens(ctx context.Context, tokens []string, maxAttempts int) (map[string]bool, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	exhausted := make(map[string]bool, len(tokens))
	const batchSize = 500

	for start := 0; start < len(tokens); start += batchSize {
		end := start + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		batch := tokens[start:end]

		placeholders := make([]string, len(batch))
		args := make([]any, len(batch)+1)
		for i, t := range batch {
			args[i] = t
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		args[len(batch)] = maxAttempts

		q := `SELECT DISTINCT token FROM listing_history
			WHERE token IN (` + strings.Join(placeholders, ", ") + `)
			  AND enrich_attempts >= $` + fmt.Sprintf("%d", len(batch)+1)

		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("list enrich exhausted tokens: %w", err)
		}

		for rows.Next() {
			var tok string
			if err := rows.Scan(&tok); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan exhausted token: %w", err)
			}
			exhausted[tok] = true
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("exhausted tokens rows: %w", err)
		}
		_ = rows.Close()
	}
	return exhausted, nil
}

func (s *Store) ResetUnenrichedAttempts(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE listing_history SET enrich_attempts = 0
		 WHERE (`+unenrichedMissingFieldsWhereSQL+`)
		   AND enrich_attempts > 0
		   AND enrich_attempts < 30`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// PruneListings drops listings nobody has seen for olderThan — measured from
// the last time the source still served them, not from when we first found
// them. A car can sit on Yad2 for longer than the retention window; pruning by
// first_seen_at deleted such a listing while it was still live, and the next
// ingest re-inserted it with a fresh first_seen_at — so it resurfaced in the UI
// as newly found and lost its enrichment counters. Bookmarked listings are
// exempt, as before.
func (s *Store) PruneListings(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM listing_history
		WHERE last_seen_at < $1
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
		SELECT lh.token, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.body_type, lh.year, lh.price,
			lh.km, lh.hand, lh.city, lh.page_link, lh.image_url,
			lh.engine_volume, lh.horse_power, lh.engine_type, lh.gear_box, lh.description,
			lh.is_commercial, lh.fitness_score, lh.median_price, lh.cohort_size, lh.deal_score, lh.base_price, lh.first_seen_at, lh.posted_at, lh.removed_at,
			lh.score_condition, lh.score_value, lh.score_engine, lh.original_ownership
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
