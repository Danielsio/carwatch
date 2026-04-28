package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) DBFileSize() (int64, error) {
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
	tables := []string{
		"users",
		"searches",
		"listing_history",
		"price_history",
		"dedup_seen",
		"notifications",
		"market_cache",
		"catalog",
	}
	sizes := make(map[string]int64, len(tables))
	for _, t := range tables {
		var count int64
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+t).Scan(&count)
		if err != nil {
			continue
		}
		sizes[t] = count
	}
	return sizes, nil
}
