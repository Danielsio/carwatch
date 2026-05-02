package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) DBFileSize() (int64, error) {
	if s.dbPath == ":memory:" || strings.HasPrefix(s.dbPath, "file::memory:") {
		return 0, nil
	}

	var total int64
	for _, name := range []string{s.dbPath, s.dbPath + "-wal", s.dbPath + "-shm"} {
		fi, err := os.Stat(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, fmt.Errorf("stat %s: %w", filepath.Base(name), err)
		}
		total += fi.Size()
	}
	return total, nil
}

func (s *Store) CountAllListings(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM listing_history").Scan(&count)
	return count, err
}

func (s *Store) TableSizes(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}

	sizes := make(map[string]int64, len(tables))
	for _, t := range tables {
		var count int64
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM \""+t+"\"").Scan(&count); err != nil {
			return nil, fmt.Errorf("count rows for table %s: %w", t, err)
		}
		sizes[t] = count
	}
	return sizes, nil
}

var purgeable = map[string]bool{
	"listing_history":       true,
	"price_history":         true,
	"dedup_seen":            true,
	"seen_listings":         true,
	"notifications":         true,
	"pending_notifications": true,
	"market_cache":          true,
	"catalog_cache":         true,
	"saved_listings":        true,
	"hidden_listings":       true,
	"pending_digest":        true,
}

func (s *Store) PurgeTable(ctx context.Context, table string) (int64, error) {
	if !purgeable[table] {
		return 0, fmt.Errorf("table %q is not purgeable", table)
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM \""+table+"\"")
	if err != nil {
		return 0, fmt.Errorf("purge %s: %w", table, err)
	}
	return result.RowsAffected()
}

func (s *Store) AdminListListings(ctx context.Context, limit, offset int) ([]storage.ListingRecord, int64, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM listing_history").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT token, chat_id, search_id, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		ORDER BY first_seen_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query listings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.ListingRecord
	for rows.Next() {
		var r storage.ListingRecord
		var score *float64
		var firstSeen string
		if err := rows.Scan(
			&r.Token, &r.ChatID, &r.SearchID, &r.SearchName,
			&r.Manufacturer, &r.Model, &r.Year, &r.Price,
			&r.Km, &r.Hand, &r.City, &r.PageLink, &r.ImageURL,
			&score, &firstSeen,
		); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		r.FitnessScore = score
		parsed, parseErr := parseFlexibleTime(firstSeen)
		if parseErr != nil {
			return nil, 0, fmt.Errorf("parse first_seen_at %q for token %s: %w", firstSeen, r.Token, parseErr)
		}
		r.FirstSeenAt = parsed
		items = append(items, r)
	}
	return items, total, rows.Err()
}

var timeFormats = []string{
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
}

func parseFlexibleTime(s string) (time.Time, error) {
	for _, layout := range timeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

func (s *Store) AdminDeleteListing(ctx context.Context, token string, chatID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM listing_history WHERE token = ? AND chat_id = ?", token, chatID)
	return err
}

func (s *Store) VacuumDB(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("wal checkpoint: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("post-vacuum wal checkpoint: %w", err)
	}
	return nil
}
