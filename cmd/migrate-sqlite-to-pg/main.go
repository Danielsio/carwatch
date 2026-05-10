package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	sqlitePath := flag.String("sqlite", "", "path to SQLite database")
	pgDSN := flag.String("pg", "", "PostgreSQL DSN (e.g. postgres://carwatch:carwatch@localhost:5432/carwatch?sslmode=disable)")
	flag.Parse()

	if *sqlitePath == "" || *pgDSN == "" {
		fmt.Fprintln(os.Stderr, "Usage: migrate-sqlite-to-pg -sqlite <path> -pg <dsn>")
		os.Exit(1)
	}

	ctx := context.Background()

	lite, err := sql.Open("sqlite3", *sqlitePath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = lite.Close() }()

	pg, err := sql.Open("pgx", *pgDSN)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = pg.Close() }()

	if err := pg.PingContext(ctx); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	tables := []tableSpec{
		{name: "users", columns: "chat_id, username, state, state_data, created_at, active, digest_mode, digest_interval, digest_last_flushed, language, tier, tier_expires_at, trial_used, daily_digest, daily_digest_time, daily_digest_last_sent, channel, channel_id, linked_web_id, last_seen_at", count: 20},
		{name: "searches", columns: "id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, seller_filter, active, created_at, share_token", count: 19},
		{name: "seen_listings", columns: "token, chat_id, search_id, first_seen_at", count: 4},
		{name: "pending_notifications", columns: "id, recipient, search_name, payload, created_at", count: 5},
		{name: "price_history", columns: "id, token, price, observed_at", count: 4},
		{name: "listing_history", columns: "token, chat_id, search_id, search_name, manufacturer, model, sub_model, year, price, km, hand, city, page_link, image_url, engine_volume, horse_power, engine_type, gear_box, description, is_commercial, fitness_score, first_seen_at", count: 22},
		{name: "pending_digest", columns: "id, chat_id, listing_payload, created_at", count: 4},
		{name: "saved_listings", columns: "chat_id, token, saved_at", count: 3},
		{name: "hidden_listings", columns: "chat_id, token, hidden_at", count: 3},
		{name: "link_tokens", columns: "token, web_chat_id, created_at, expires_at, used", count: 5},
	}

	for _, t := range tables {
		n, err := migrateTable(ctx, lite, pg, t)
		if err != nil {
			log.Fatalf("migrate %s: %v", t.name, err)
		}
		log.Printf("migrated %s: %d rows", t.name, n)
	}

	if err := resetSequences(ctx, pg); err != nil {
		log.Fatalf("reset sequences: %v", err)
	}
	log.Println("sequences reset successfully")

	log.Println("migration complete")
}

type tableSpec struct {
	name    string
	columns string
	count   int
}

func migrateTable(ctx context.Context, lite, pg *sql.DB, spec tableSpec) (int64, error) {
	rows, err := lite.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s", spec.columns, spec.name))
	if err != nil {
		return 0, fmt.Errorf("query sqlite: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tx, err := pg.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin pg tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := buildPlaceholders(spec.count)
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", spec.name, spec.columns, placeholders)

	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	var total int64
	for rows.Next() {
		vals := make([]any, spec.count)
		ptrs := make([]any, spec.count)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return 0, fmt.Errorf("scan row: %w", err)
		}

		for i, v := range vals {
			vals[i] = convertValue(v)
		}

		if _, err := stmt.ExecContext(ctx, vals...); err != nil {
			return 0, fmt.Errorf("insert row: %w", err)
		}
		total++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate rows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return total, nil
}

func buildPlaceholders(n int) string {
	s := ""
	for i := 1; i <= n; i++ {
		if i > 1 {
			s += ", "
		}
		s += fmt.Sprintf("$%d", i)
	}
	return s
}

func convertValue(v any) any {
	switch val := v.(type) {
	case string:
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.000000",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
		} {
			if t, err := time.Parse(layout, val); err == nil {
				return t.UTC()
			}
		}
		return val
	case []byte:
		return string(val)
	default:
		return v
	}
}

func resetSequences(ctx context.Context, pg *sql.DB) error {
	seqs := []struct {
		table, column string
	}{
		{"searches", "id"},
		{"pending_notifications", "id"},
		{"price_history", "id"},
		{"pending_digest", "id"},
	}
	for _, s := range seqs {
		_, err := pg.ExecContext(ctx, fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE(MAX(%s), 0) + 1, false) FROM %s`,
			s.table, s.column, s.column, s.table))
		if err != nil {
			return fmt.Errorf("reset sequence for %s.%s: %w", s.table, s.column, err)
		}
	}
	return nil
}
