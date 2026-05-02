package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) SaveListing(ctx context.Context, r storage.ListingRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO listing_history
		(token, chat_id, search_id, search_name, manufacturer, model, year, price, km, hand, city, page_link, image_url, fitness_score, first_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token, chat_id) DO UPDATE SET
			search_id = CASE WHEN excluded.search_id > 0 THEN excluded.search_id ELSE listing_history.search_id END,
			search_name = CASE WHEN excluded.search_id > 0 THEN excluded.search_name ELSE listing_history.search_name END,
			price = excluded.price,
			km = excluded.km,
			hand = excluded.hand,
			image_url = CASE WHEN excluded.image_url != '' THEN excluded.image_url ELSE listing_history.image_url END,
			fitness_score = excluded.fitness_score`,
		r.Token, r.ChatID, r.SearchID, r.SearchName, r.Manufacturer, r.Model, r.Year, r.Price,
		r.Km, r.Hand, r.City, r.PageLink, r.ImageURL, r.FitnessScore, r.FirstSeenAt.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	if r.Manufacturer != "" && r.Model != "" && r.Year > 0 && r.Price > 0 {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO market_cache (token, manufacturer, model, year, price)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(token) DO UPDATE SET
				manufacturer = excluded.manufacturer,
				model = excluded.model,
				year = excluded.year,
				price = excluded.price,
				updated_at = CURRENT_TIMESTAMP`,
			r.Token, r.Manufacturer, r.Model, r.Year, r.Price); err != nil {
			return fmt.Errorf("upsert market_cache: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveListings(ctx context.Context, records []storage.ListingRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	listingStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO listing_history
		(token, chat_id, search_id, search_name, manufacturer, model, year, price, km, hand, city, page_link, image_url, fitness_score, first_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token, chat_id) DO UPDATE SET
			search_id = CASE WHEN excluded.search_id > 0 THEN excluded.search_id ELSE listing_history.search_id END,
			search_name = CASE WHEN excluded.search_id > 0 THEN excluded.search_name ELSE listing_history.search_name END,
			price = excluded.price,
			km = excluded.km,
			hand = excluded.hand,
			image_url = CASE WHEN excluded.image_url != '' THEN excluded.image_url ELSE listing_history.image_url END,
			fitness_score = excluded.fitness_score`)
	if err != nil {
		return err
	}
	defer func() { _ = listingStmt.Close() }()

	marketStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO market_cache (token, manufacturer, model, year, price)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET
			manufacturer = excluded.manufacturer,
			model = excluded.model,
			year = excluded.year,
			price = excluded.price,
			updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer func() { _ = marketStmt.Close() }()

	for _, r := range records {
		if _, err := listingStmt.ExecContext(ctx,
			r.Token, r.ChatID, r.SearchID, r.SearchName, r.Manufacturer, r.Model, r.Year, r.Price,
			r.Km, r.Hand, r.City, r.PageLink, r.ImageURL, r.FitnessScore, r.FirstSeenAt.UTC().Format("2006-01-02 15:04:05")); err != nil {
			return err
		}
		if r.Manufacturer != "" && r.Model != "" && r.Year > 0 && r.Price > 0 {
			if _, err := marketStmt.ExecContext(ctx,
				r.Token, r.Manufacturer, r.Model, r.Year, r.Price); err != nil {
				return fmt.Errorf("upsert market_cache: %w", err)
			}
		}
	}
	return tx.Commit()
}

func (s *Store) GetListing(ctx context.Context, chatID int64, token string) (*storage.ListingRecord, error) {
	var l storage.ListingRecord
	var fs sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = ? AND token = ?
		ORDER BY rowid DESC LIMIT 1`, chatID, token).
		Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if fs.Valid {
		l.FitnessScore = &fs.Float64
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
		var l storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
			return nil, err
		}
		if fs.Valid {
			l.FitnessScore = &fs.Float64
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
		var l storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
			return nil, err
		}
		if fs.Valid {
			l.FitnessScore = &fs.Float64
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) ListSearchListings(ctx context.Context, chatID int64, searchID int64, limit, offset int, sort string) ([]storage.ListingRecord, error) {
	orderBy := "first_seen_at DESC, token DESC"
	switch sort {
	case "newest":
		orderBy = "first_seen_at DESC, token DESC"
	case "price_asc":
		orderBy = "price ASC, token DESC"
	case "price_desc":
		orderBy = "price DESC, token DESC"
	case "score":
		orderBy = "fitness_score DESC NULLS LAST, token DESC"
	case "km":
		orderBy = "km ASC, token DESC"
	case "year":
		orderBy = "year DESC, token DESC"
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = ? AND search_id = ?
		ORDER BY %s
		LIMIT ? OFFSET ?`, orderBy), chatID, searchID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		var l storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
			return nil, err
		}
		if fs.Valid {
			l.FitnessScore = &fs.Float64
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) CountSearchListings(ctx context.Context, chatID int64, searchID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM listing_history
		WHERE chat_id = ? AND search_id = ?`, chatID, searchID).Scan(&count)
	return count, err
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
		var l storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
			return nil, err
		}
		if fs.Valid {
			l.FitnessScore = &fs.Float64
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

	args := make([]interface{}, 0, 1+len(uniq))
	args = append(args, chatID)
	for _, t := range uniq {
		args = append(args, t)
	}
	placeholders := ""
	for i := range uniq {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT token FROM saved_listings WHERE chat_id = ? AND token IN ("+placeholders+")",
		args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]bool)
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return nil, err
		}
		out[tok] = true
	}
	return out, rows.Err()
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
