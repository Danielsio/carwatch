package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) CreateSearch(ctx context.Context, search storage.Search) (int64, error) {
	source := search.Source
	if source == "" {
		source = "yad2"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var nextSeq int
	err = tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(user_seq), 0) + 1 FROM searches WHERE chat_id = ?",
		search.ChatID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("next user_seq: %w", err)
	}

	shareToken, err := generateShareToken()
	if err != nil {
		return 0, fmt.Errorf("generate share token: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO searches (chat_id, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, user_seq, share_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		search.ChatID, search.Name, source, search.Manufacturer, search.Model,
		search.YearMin, search.YearMax, search.PriceMax,
		search.EngineMinCC, search.MaxKm, search.MaxHand,
		search.Keywords, search.ExcludeKeys, nextSeq, shareToken)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return id, nil
}

func (s *Store) ListSearches(ctx context.Context, chatID int64) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE chat_id = ? ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSearches(rows)
}

func (s *Store) GetSearch(ctx context.Context, id int64) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE id = ?`, id)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys,
		&search.Active, &search.CreatedAt, &search.ShareToken)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &search, nil
}

func (s *Store) GetSearchBySeq(ctx context.Context, chatID int64, seq int) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE chat_id = ? AND user_seq = ?`, chatID, seq)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys,
		&search.Active, &search.CreatedAt, &search.ShareToken)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &search, nil
}

func (s *Store) GetSearchByShareToken(ctx context.Context, token string) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE share_token = ?`, token)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys,
		&search.Active, &search.CreatedAt, &search.ShareToken)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &search, nil
}

func (s *Store) UpdateSearch(ctx context.Context, search storage.Search) error {
	source := search.Source
	if source == "" {
		source = "yad2"
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE searches SET name=?, source=?, manufacturer=?, model=?,
			year_min=?, year_max=?, price_max=?, engine_min_cc=?,
			max_km=?, max_hand=?, keywords=?, exclude_keys=?
		WHERE id=? AND chat_id=?`,
		search.Name, source, search.Manufacturer, search.Model,
		search.YearMin, search.YearMax, search.PriceMax, search.EngineMinCC,
		search.MaxKm, search.MaxHand, search.Keywords, search.ExcludeKeys,
		search.ID, search.ChatID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteSearch(ctx context.Context, id int64, chatID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var searchName string
	err = tx.QueryRowContext(ctx,
		"SELECT name FROM searches WHERE id = ? AND chat_id = ?", id, chatID,
	).Scan(&searchName)
	if err == sql.ErrNoRows {
		return storage.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("fetch search name: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM seen_listings WHERE search_id = ? AND chat_id = ?",
		id, chatID); err != nil {
		return fmt.Errorf("delete seen_listings: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM listing_history WHERE search_name = ? AND chat_id = ?",
		searchName, chatID); err != nil {
		return fmt.Errorf("delete listing_history: %w", err)
	}

	recipientStr := fmt.Sprintf("%d", chatID)
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM pending_notifications WHERE search_name = ? AND recipient = ?",
		searchName, recipientStr); err != nil {
		return fmt.Errorf("delete pending_notifications: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM searches WHERE id = ? AND chat_id = ?",
		id, chatID); err != nil {
		return fmt.Errorf("delete search: %w", err)
	}

	return tx.Commit()
}

func (s *Store) SetSearchActive(ctx context.Context, id int64, chatID int64, active bool) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE searches SET active = ? WHERE id = ? AND chat_id = ?", active, id, chatID)
	return err
}

func (s *Store) ListAllActiveSearches(ctx context.Context) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.chat_id, s.user_seq, s.name, s.source, s.manufacturer, s.model, s.year_min, s.year_max, s.price_max, s.engine_min_cc, s.max_km, s.max_hand, s.keywords, s.exclude_keys, s.active, s.created_at, COALESCE(s.share_token, '')
		FROM searches s
		JOIN users u ON s.chat_id = u.chat_id
		WHERE s.active = true AND u.active = true
		ORDER BY s.source, s.manufacturer, s.model`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSearches(rows)
}

func (s *Store) CountSearches(ctx context.Context, chatID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM searches WHERE chat_id = ? AND active = true",
		chatID).Scan(&count)
	return count, err
}

func (s *Store) CountAllSearches(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM searches WHERE active = true").Scan(&count)
	return count, err
}

func scanSearches(rows *sql.Rows) ([]storage.Search, error) {
	var searches []storage.Search
	for rows.Next() {
		var s storage.Search
		if err := rows.Scan(&s.ID, &s.ChatID, &s.UserSeq, &s.Name, &s.Source, &s.Manufacturer, &s.Model,
			&s.YearMin, &s.YearMax, &s.PriceMax,
			&s.EngineMinCC, &s.MaxKm, &s.MaxHand,
			&s.Keywords, &s.ExcludeKeys,
			&s.Active, &s.CreatedAt, &s.ShareToken); err != nil {
			return nil, err
		}
		searches = append(searches, s)
	}
	return searches, rows.Err()
}
