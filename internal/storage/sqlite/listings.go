package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

const firstSeenAtLayout = "2006-01-02 15:04:05.000000"

const upsertListingSQL = `
	INSERT INTO listing_history
	(token, chat_id, search_id, search_name, manufacturer, model, year, price, km, hand, city, page_link, image_url, fitness_score, first_seen_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(token, chat_id) DO UPDATE SET
		search_id = CASE WHEN excluded.search_id > 0 THEN excluded.search_id ELSE listing_history.search_id END,
		search_name = CASE WHEN excluded.search_id > 0 THEN excluded.search_name ELSE listing_history.search_name END,
		manufacturer = CASE WHEN excluded.manufacturer != '' THEN excluded.manufacturer ELSE listing_history.manufacturer END,
		model = CASE WHEN excluded.model != '' THEN excluded.model ELSE listing_history.model END,
		year = CASE WHEN excluded.year > 0 THEN excluded.year ELSE listing_history.year END,
		price = excluded.price,
		km = CASE WHEN excluded.km > 0 THEN excluded.km ELSE listing_history.km END,
		hand = excluded.hand,
		city = CASE WHEN excluded.city != '' THEN excluded.city ELSE listing_history.city END,
		page_link = CASE WHEN excluded.page_link != '' THEN excluded.page_link ELSE listing_history.page_link END,
		image_url = CASE WHEN excluded.image_url != '' THEN excluded.image_url ELSE listing_history.image_url END,
		fitness_score = excluded.fitness_score`

type listingScanner interface {
	Scan(dest ...any) error
}

func scanListingRow(sc listingScanner) (storage.ListingRecord, error) {
	var l storage.ListingRecord
	var fs sql.NullFloat64
	if err := sc.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
		&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
		return l, err
	}
	if fs.Valid {
		l.FitnessScore = &fs.Float64
	}
	return l, nil
}

func upsertListingArgs(r storage.ListingRecord) []any {
	return []any{
		r.Token, r.ChatID, r.SearchID, r.SearchName, r.Manufacturer, r.Model, r.Year, r.Price,
		r.Km, r.Hand, r.City, r.PageLink, r.ImageURL, r.FitnessScore, r.FirstSeenAt.UTC().Format(firstSeenAtLayout),
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

const backfillListingSQL_sqlite = `
	UPDATE listing_history SET
		km = CASE WHEN ?2 > 0 AND listing_history.km <= 0 THEN ?2 ELSE listing_history.km END,
		city = CASE WHEN ?3 != '' AND listing_history.city = '' THEN ?3 ELSE listing_history.city END,
		image_url = CASE WHEN ?4 != '' AND listing_history.image_url = '' THEN ?4 ELSE listing_history.image_url END
	WHERE token = ?1 AND (listing_history.km <= 0 OR listing_history.city = '' OR listing_history.image_url = '')`

func (s *Store) BackfillListings(ctx context.Context, records []storage.ListingRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("backfill begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, backfillListingSQL_sqlite)
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
			placeholders[i] = "?"
		}

		q := `SELECT token, MAX(km), MAX(city), MAX(image_url)
			FROM listing_history
			WHERE token IN (` + strings.Join(placeholders, ", ") + `) AND km > 0
			GROUP BY token`

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
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = ? AND token = ?
		ORDER BY rowid DESC LIMIT 1`, chatID, token)
	l, err := scanListingRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (s *Store) ListUserListings(ctx context.Context, chatID int64, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = ?
		ORDER BY first_seen_at DESC, token DESC
		LIMIT ? OFFSET ?`, chatID, limit, offset)
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
		WHERE chat_id = ?`, chatID).Scan(&count)
	return count, err
}

func (s *Store) ListListings(ctx context.Context, limit int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, year, price, km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE rowid IN (SELECT MAX(rowid) FROM listing_history GROUP BY token)
		ORDER BY first_seen_at DESC LIMIT ?`, limit)
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

func buildFilterClauses(f storage.ListingFilter) (string, []any) {
	var clauses []string
	var args []any
	if f.PriceMax > 0 {
		clauses = append(clauses, "price <= ?")
		args = append(args, f.PriceMax)
	}
	if f.YearMin > 0 {
		clauses = append(clauses, "year >= ?")
		args = append(args, f.YearMin)
	}
	if f.YearMax > 0 {
		clauses = append(clauses, "year <= ?")
		args = append(args, f.YearMax)
	}
	if f.MaxKm > 0 {
		clauses = append(clauses, "km > 0 AND km <= ?")
		args = append(args, f.MaxKm)
	}
	if f.MaxHand > 0 {
		clauses = append(clauses, "hand <= ?")
		args = append(args, f.MaxHand)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
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

	filterSQL, filterArgs := buildFilterClauses(f)
	args := []any{chatID, searchID}
	args = append(args, filterArgs...)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = ? AND search_id = ?%s
		ORDER BY %s
		LIMIT ? OFFSET ?`, filterSQL, orderBy), args...)
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
	filterSQL, filterArgs := buildFilterClauses(f)
	args := []any{chatID, searchID}
	args = append(args, filterArgs...)

	var count int64
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM listing_history
		WHERE chat_id = ? AND search_id = ?%s`, filterSQL), args...).Scan(&count)
	return count, err
}

func (s *Store) PruneListings(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM listing_history
		WHERE first_seen_at < ?
		  AND NOT EXISTS (
		      SELECT 1 FROM saved_listings
		      WHERE saved_listings.token = listing_history.token
		        AND saved_listings.chat_id = listing_history.chat_id
		  )`,
		cutoff.UTC().Format(firstSeenAtLayout))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) SaveBookmark(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO saved_listings (chat_id, token) VALUES (?, ?)",
		chatID, token)
	return err
}

func (s *Store) RemoveBookmark(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM saved_listings WHERE chat_id = ? AND token = ?",
		chatID, token)
	return err
}

func (s *Store) ListSaved(ctx context.Context, chatID int64, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT lh.token, lh.search_name, lh.manufacturer, lh.model, lh.year, lh.price,
			lh.km, lh.hand, lh.city, lh.page_link, lh.image_url, lh.fitness_score, lh.first_seen_at
		FROM saved_listings sl
		JOIN listing_history lh ON sl.token = lh.token AND sl.chat_id = lh.chat_id
		WHERE sl.chat_id = ?
		ORDER BY sl.saved_at DESC
		LIMIT ? OFFSET ?`, chatID, limit, offset)
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
		"SELECT COUNT(*) FROM saved_listings WHERE chat_id = ?", chatID).Scan(&count)
	return count, err
}

func (s *Store) IsSaved(ctx context.Context, chatID int64, token string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM saved_listings WHERE chat_id = ? AND token = ?",
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

		args := make([]interface{}, 0, 1+len(batch))
		args = append(args, chatID)
		for _, t := range batch {
			args = append(args, t)
		}
		placeholders := ""
		for i := range batch {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
		}

		rows, err := tx.QueryContext(ctx,
			"SELECT token FROM saved_listings WHERE chat_id = ? AND token IN ("+placeholders+")",
			args...)
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
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO hidden_listings (chat_id, token) VALUES (?, ?)",
		chatID, token)
	return err
}

func (s *Store) UnhideListing(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM hidden_listings WHERE chat_id = ? AND token = ?",
		chatID, token)
	return err
}

func (s *Store) IsHidden(ctx context.Context, chatID int64, token string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM hidden_listings WHERE chat_id = ? AND token = ?",
		chatID, token).Scan(&count)
	return count > 0, err
}

func (s *Store) ListHidden(ctx context.Context, chatID int64, limit, offset int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT token FROM hidden_listings WHERE chat_id = ? ORDER BY hidden_at DESC LIMIT ? OFFSET ?",
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
		"SELECT COUNT(*) FROM hidden_listings WHERE chat_id = ?", chatID).Scan(&count)
	return count, err
}

func (s *Store) ClearHidden(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM hidden_listings WHERE chat_id = ?", chatID)
	return err
}

func (s *Store) ListHiddenTokens(ctx context.Context, chatID int64) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT token FROM hidden_listings WHERE chat_id = ?", chatID)
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
