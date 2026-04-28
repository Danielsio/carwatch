package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
