package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

// generateShareToken returns a 32-character hex string from 16 random bytes.
func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

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
		`SELECT COALESCE(MAX(user_seq), 0) + 1 FROM searches WHERE chat_id = $1`,
		search.ChatID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("next user_seq: %w", err)
	}

	shareToken, err := generateShareToken()
	if err != nil {
		return 0, fmt.Errorf("generate share token: %w", err)
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO searches (chat_id, name, source, manufacturer, model, year_min, year_max, price_min, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, seller_filter, gear_box, price_only, photo_only, user_seq, share_token)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id`,
		search.ChatID, search.Name, source, search.Manufacturer, search.Model,
		search.YearMin, search.YearMax, search.PriceMin, search.PriceMax,
		search.EngineMinCC, search.MaxKm, search.MaxHand,
		search.Keywords, search.ExcludeKeys, storage.NormalizeSellerFilter(search.SellerFilter),
		search.GearBox, search.PriceOnly, search.PhotoOnly, nextSeq, shareToken).Scan(&id)
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
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, COALESCE(price_min, 0), price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, COALESCE(seller_filter, 'any'), COALESCE(gear_box, ''), price_only, photo_only, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE chat_id = $1 ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list searches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanSearches(rows)
}

func (s *Store) GetSearch(ctx context.Context, id int64, chatID int64) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, COALESCE(price_min, 0), price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, COALESCE(seller_filter, 'any'), COALESCE(gear_box, ''), price_only, photo_only, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE id = $1 AND chat_id = $2`, id, chatID)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMin, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys, &search.SellerFilter,
		&search.GearBox, &search.PriceOnly, &search.PhotoOnly,
		&search.Active, &search.CreatedAt, &search.ShareToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search: %w", err)
	}
	return &search, nil
}

func (s *Store) GetSearchBySeq(ctx context.Context, chatID int64, seq int) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, COALESCE(price_min, 0), price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, COALESCE(seller_filter, 'any'), COALESCE(gear_box, ''), price_only, photo_only, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE chat_id = $1 AND user_seq = $2`, chatID, seq)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMin, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys, &search.SellerFilter,
		&search.GearBox, &search.PriceOnly, &search.PhotoOnly,
		&search.Active, &search.CreatedAt, &search.ShareToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search by seq: %w", err)
	}
	return &search, nil
}

func (s *Store) GetSearchByShareToken(ctx context.Context, token string) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, COALESCE(price_min, 0), price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, COALESCE(seller_filter, 'any'), COALESCE(gear_box, ''), price_only, photo_only, active, created_at, COALESCE(share_token, '')
		FROM searches WHERE share_token = $1`, token)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMin, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys, &search.SellerFilter,
		&search.GearBox, &search.PriceOnly, &search.PhotoOnly,
		&search.Active, &search.CreatedAt, &search.ShareToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search by share token: %w", err)
	}
	return &search, nil
}

func (s *Store) UpdateSearch(ctx context.Context, search storage.Search) error {
	source := search.Source
	if source == "" {
		source = "yad2"
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE searches SET name=$1, source=$2, manufacturer=$3, model=$4,
			year_min=$5, year_max=$6, price_min=$7, price_max=$8, engine_min_cc=$9,
			max_km=$10, max_hand=$11, keywords=$12, exclude_keys=$13, seller_filter=$14,
			gear_box=$15, price_only=$16, photo_only=$17
		WHERE id=$18 AND chat_id=$19`,
		search.Name, source, search.Manufacturer, search.Model,
		search.YearMin, search.YearMax, search.PriceMin, search.PriceMax, search.EngineMinCC,
		search.MaxKm, search.MaxHand, search.Keywords, search.ExcludeKeys,
		storage.NormalizeSellerFilter(search.SellerFilter),
		search.GearBox, search.PriceOnly, search.PhotoOnly,
		search.ID, search.ChatID)
	if err != nil {
		return fmt.Errorf("update search: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update search rows affected: %w", err)
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
		`SELECT name FROM searches WHERE id = $1 AND chat_id = $2`, id, chatID,
	).Scan(&searchName)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("fetch search name: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM seen_listings WHERE search_id = $1 AND chat_id = $2`,
		id, chatID); err != nil {
		return fmt.Errorf("delete seen_listings: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM listing_history WHERE search_id = $1 AND chat_id = $2`,
		id, chatID); err != nil {
		return fmt.Errorf("delete listing_history: %w", err)
	}

	recipientStr := fmt.Sprintf("%d", chatID)
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM pending_notifications WHERE search_name = $1 AND recipient = $2`,
		searchName, recipientStr); err != nil {
		return fmt.Errorf("delete pending_notifications by name: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM searches WHERE id = $1 AND chat_id = $2`,
		id, chatID); err != nil {
		return fmt.Errorf("delete search: %w", err)
	}

	return tx.Commit()
}

func (s *Store) SetSearchActive(ctx context.Context, id int64, chatID int64, active bool) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE searches SET active = $1 WHERE id = $2 AND chat_id = $3`, active, id, chatID)
	if err != nil {
		return fmt.Errorf("set search active: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set search active rows affected: %w", err)
	}
	if rows == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) ListAllActiveSearches(ctx context.Context) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.chat_id, s.user_seq, s.name, s.source, s.manufacturer, s.model, s.year_min, s.year_max, COALESCE(s.price_min, 0), s.price_max, s.engine_min_cc, s.max_km, s.max_hand, s.keywords, s.exclude_keys, COALESCE(s.seller_filter, 'any'), COALESCE(s.gear_box, ''), s.price_only, s.photo_only, s.active, s.created_at, COALESCE(s.share_token, '')
		FROM searches s
		JOIN users u ON s.chat_id = u.chat_id
		WHERE s.active = true AND u.active = true
		ORDER BY s.source, s.manufacturer, s.model`)
	if err != nil {
		return nil, fmt.Errorf("list active searches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanSearches(rows)
}

func (s *Store) CountSearches(ctx context.Context, chatID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM searches WHERE chat_id = $1 AND active = true`,
		chatID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count searches: %w", err)
	}
	return count, nil
}

func (s *Store) CountAllSearches(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM searches WHERE active = true`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count all searches: %w", err)
	}
	return count, nil
}

func scanSearches(rows *sql.Rows) ([]storage.Search, error) {
	var searches []storage.Search
	for rows.Next() {
		var s storage.Search
		if err := rows.Scan(&s.ID, &s.ChatID, &s.UserSeq, &s.Name, &s.Source, &s.Manufacturer, &s.Model,
			&s.YearMin, &s.YearMax, &s.PriceMin, &s.PriceMax,
			&s.EngineMinCC, &s.MaxKm, &s.MaxHand,
			&s.Keywords, &s.ExcludeKeys, &s.SellerFilter,
			&s.GearBox, &s.PriceOnly, &s.PhotoOnly,
			&s.Active, &s.CreatedAt, &s.ShareToken); err != nil {
			return nil, err
		}
		searches = append(searches, s)
	}
	return searches, rows.Err()
}
