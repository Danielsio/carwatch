package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) SaveCatalogEntries(ctx context.Context, entries []storage.CatalogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Mark all existing entries as stale.
	if _, err := tx.ExecContext(ctx,
		"UPDATE catalog_cache SET updated_at = datetime('now', '-1 year')"); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO catalog_cache (manufacturer_id, manufacturer_name, model_id, model_name, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(manufacturer_id, model_id) DO UPDATE SET
			manufacturer_name = excluded.manufacturer_name,
			model_name = excluded.model_name,
			updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, e.ManufacturerID, e.ManufacturerName, e.ModelID, e.ModelName); err != nil {
			return err
		}
	}

	// Remove entries not refreshed in this batch.
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM catalog_cache WHERE updated_at < datetime('now', '-1 hour')"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) LoadCatalogEntries(ctx context.Context) ([]storage.CatalogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT manufacturer_id, manufacturer_name, model_id, model_name FROM catalog_cache ORDER BY manufacturer_name, model_name")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []storage.CatalogEntry
	for rows.Next() {
		var e storage.CatalogEntry
		if err := rows.Scan(&e.ManufacturerID, &e.ManufacturerName, &e.ModelID, &e.ModelName); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) CatalogAge(ctx context.Context) (time.Duration, error) {
	var raw sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT MIN(updated_at) FROM catalog_cache").Scan(&raw)
	if err != nil {
		return 0, err
	}
	if !raw.Valid || raw.String == "" {
		return time.Duration(1<<63 - 1), nil
	}
	updatedAt, err := time.Parse("2006-01-02 15:04:05", raw.String)
	if err != nil {
		return 0, fmt.Errorf("parse updated_at: %w", err)
	}
	return time.Since(updatedAt), nil
}
