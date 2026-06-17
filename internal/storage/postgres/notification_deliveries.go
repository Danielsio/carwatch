package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

const recordDeliverySQL = `
	INSERT INTO notification_deliveries
		(chat_id, token, search_id, alert_type, channel, status, batch_id, error)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (chat_id, token, alert_type, channel) WHERE token <> ''
	DO UPDATE SET
		status     = EXCLUDED.status,
		search_id  = CASE WHEN EXCLUDED.search_id > 0 THEN EXCLUDED.search_id ELSE notification_deliveries.search_id END,
		batch_id   = CASE WHEN EXCLUDED.batch_id <> '' THEN EXCLUDED.batch_id ELSE notification_deliveries.batch_id END,
		error      = EXCLUDED.error,
		attempts   = notification_deliveries.attempts + 1,
		updated_at = NOW()`

// RecordDeliveries upserts each delivery outcome. token = '' rows (aggregate
// sends) never match the partial unique index, so they always insert.
func (s *Store) RecordDeliveries(ctx context.Context, events []storage.DeliveryEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("record deliveries begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, recordDeliverySQL)
	if err != nil {
		return fmt.Errorf("record deliveries prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, e := range events {
		channel := e.Channel
		if channel == "" {
			channel = "telegram"
		}
		status := e.Status
		if status == "" {
			status = "sent"
		}
		if _, err := stmt.ExecContext(ctx,
			e.ChatID, e.Token, e.SearchID, e.AlertType, channel, status, e.BatchID, e.Error); err != nil {
			return fmt.Errorf("record deliveries exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("record deliveries commit: %w", err)
	}
	return nil
}

// recordDigestDeliveries inserts a 'digest' delivery row for every distinct
// listing token covered by the pending_digest items for chatID at or before
// `before`. It must run inside the AckDigest transaction so the delivery record
// and the queue flush commit atomically.
func (s *Store) recordDigestDeliveries(ctx context.Context, tx *sql.Tx, chatID int64, before time.Time) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT tokens FROM pending_digest WHERE chat_id = $1 AND created_at <= $2`,
		chatID, before)
	if err != nil {
		return fmt.Errorf("ack digest read tokens: %w", err)
	}
	var csvs []string
	for rows.Next() {
		var csv string
		if err := rows.Scan(&csv); err != nil {
			_ = rows.Close()
			return fmt.Errorf("ack digest scan tokens: %w", err)
		}
		csvs = append(csvs, csv)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("ack digest tokens rows: %w", err)
	}
	_ = rows.Close()

	stmt, err := tx.PrepareContext(ctx, recordDeliverySQL)
	if err != nil {
		return fmt.Errorf("ack digest prepare deliveries: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	seen := make(map[string]struct{})
	for _, csv := range csvs {
		for _, token := range strings.Split(csv, ",") {
			if token == "" {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			if _, err := stmt.ExecContext(ctx,
				chatID, token, int64(0), "digest", "telegram", "sent", "", ""); err != nil {
				return fmt.Errorf("ack digest record delivery: %w", err)
			}
		}
	}
	return nil
}

// DeliveredAmong returns the most recent delivery outcome for each of the given
// tokens that has any delivery row for chatID. Tokens with no record are absent
// from the map (caller treats absence as "not tracked / not delivered").
func (s *Store) DeliveredAmong(ctx context.Context, chatID int64, tokens []string) (map[string]storage.DeliveryInfo, error) {
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

	out := make(map[string]storage.DeliveryInfo)
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

		// DISTINCT ON keeps the newest row per token (ORDER BY token, created_at DESC).
		q := `SELECT DISTINCT ON (token) token, alert_type, status, created_at
			FROM notification_deliveries
			WHERE chat_id = $1 AND token IN (` + strings.Join(placeholders, ", ") + `)
			ORDER BY token, created_at DESC`
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var di storage.DeliveryInfo
			if err := rows.Scan(&di.Token, &di.AlertType, &di.Status, &di.SentAt); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[di.Token] = di
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, tx.Commit()
}
