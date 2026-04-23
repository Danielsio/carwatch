package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/dsionov/carwatch/internal/storage"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			chat_id     INTEGER PRIMARY KEY,
			username    TEXT NOT NULL DEFAULT '',
			state       TEXT NOT NULL DEFAULT 'idle',
			state_data  TEXT NOT NULL DEFAULT '{}',
			created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			active      BOOLEAN NOT NULL DEFAULT true
		);

		CREATE TABLE IF NOT EXISTS searches (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id       INTEGER NOT NULL REFERENCES users(chat_id),
			name          TEXT NOT NULL,
			source        TEXT NOT NULL DEFAULT 'yad2',
			manufacturer  INTEGER NOT NULL,
			model         INTEGER NOT NULL,
			year_min      INTEGER NOT NULL DEFAULT 2000,
			year_max      INTEGER NOT NULL DEFAULT 2030,
			price_max     INTEGER NOT NULL DEFAULT 9999999,
			engine_min_cc INTEGER NOT NULL DEFAULT 0,
			max_km        INTEGER NOT NULL DEFAULT 0,
			max_hand      INTEGER NOT NULL DEFAULT 0,
			active        BOOLEAN NOT NULL DEFAULT true,
			created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS seen_listings (
			token       TEXT NOT NULL,
			chat_id     INTEGER NOT NULL,
			search_id   INTEGER NOT NULL,
			first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (token, chat_id)
		);

		CREATE TABLE IF NOT EXISTS pending_notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			recipient TEXT NOT NULL,
			search_name TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS price_history (
			token TEXT NOT NULL,
			price INTEGER NOT NULL,
			observed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (token, price)
		);

		CREATE TABLE IF NOT EXISTS listing_history (
			token TEXT NOT NULL,
			chat_id INTEGER NOT NULL DEFAULT 0,
			search_name TEXT NOT NULL,
			manufacturer TEXT,
			model TEXT,
			year INTEGER,
			price INTEGER,
			km INTEGER,
			hand INTEGER,
			city TEXT,
			page_link TEXT,
			first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (token, chat_id)
		);

		CREATE TABLE IF NOT EXISTS pending_digest (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			listing_payload TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS catalog_cache (
			manufacturer_id   INTEGER NOT NULL,
			manufacturer_name TEXT NOT NULL,
			model_id          INTEGER NOT NULL,
			model_name        TEXT NOT NULL,
			updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (manufacturer_id, model_id)
		);

		CREATE INDEX IF NOT EXISTS idx_seen_listings_first_seen_at
			ON seen_listings(first_seen_at);

		CREATE INDEX IF NOT EXISTS idx_seen_listings_chatid_firstseen
			ON seen_listings(chat_id, first_seen_at DESC);
	`)
	if err != nil {
		return err
	}

	// Add digest columns to users table if they don't exist yet.
	// ALTER TABLE ADD COLUMN is idempotent-safe with the "IF NOT EXISTS" check below.
	for _, col := range []struct {
		name string
		def  string
	}{
		{"digest_mode", "TEXT NOT NULL DEFAULT 'instant'"},
		{"digest_interval", "TEXT NOT NULL DEFAULT '6h'"},
		{"digest_last_flushed", "TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00'"},
	} {
		// SQLite doesn't support IF NOT EXISTS on ALTER TABLE ADD COLUMN,
		// so we check table_info first.
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = ?", col.name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("check column %s: %w", col.name, err)
		}
		if count == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", col.name, col.def))
			if err != nil {
				return fmt.Errorf("add column %s: %w", col.name, err)
			}
		}
	}

	// Migrate listing_history to per-user: add chat_id to PK.
	var hasChatID int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('listing_history') WHERE name = 'chat_id'",
	).Scan(&hasChatID); err != nil {
		return fmt.Errorf("check listing_history chat_id: %w", err)
	}
	if hasChatID == 0 {
		if _, err := db.Exec(`
			CREATE TABLE listing_history_v2 (
				token TEXT NOT NULL,
				chat_id INTEGER NOT NULL DEFAULT 0,
				search_name TEXT NOT NULL,
				manufacturer TEXT,
				model TEXT,
				year INTEGER,
				price INTEGER,
				km INTEGER,
				hand INTEGER,
				city TEXT,
				page_link TEXT,
				first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (token, chat_id)
			);
			INSERT INTO listing_history_v2
				(token, chat_id, search_name, manufacturer, model, year, price, km, hand, city, page_link, first_seen_at)
			SELECT token, 0, search_name, manufacturer, model, year, price, km, hand, city, page_link, first_seen_at
			FROM listing_history;
			DROP TABLE listing_history;
			ALTER TABLE listing_history_v2 RENAME TO listing_history;
		`); err != nil {
			return fmt.Errorf("migrate listing_history: %w", err)
		}
	}

	return nil
}

// --- UserStore ---

func (s *Store) UpsertUser(ctx context.Context, chatID int64, username string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (chat_id, username) VALUES (?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET username = excluded.username`,
		chatID, username)
	return err
}

func (s *Store) GetUser(ctx context.Context, chatID int64) (*storage.User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT chat_id, username, state, state_data, created_at, active FROM users WHERE chat_id = ?",
		chatID)

	var u storage.User
	err := row.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpdateUserState(ctx context.Context, chatID int64, state string, stateData string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET state = ?, state_data = ? WHERE chat_id = ?",
		state, stateData, chatID)
	return err
}

func (s *Store) ListActiveUsers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT chat_id, username, state, state_data, created_at, active FROM users WHERE active = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (s *Store) SetUserActive(ctx context.Context, chatID int64, active bool) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET active = ? WHERE chat_id = ?",
		active, chatID)
	return err
}

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE active = true").Scan(&count)
	return count, err
}

func scanUsers(rows *sql.Rows) ([]storage.User, error) {
	var users []storage.User
	for rows.Next() {
		var u storage.User
		if err := rows.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- SearchStore ---

func (s *Store) CreateSearch(ctx context.Context, search storage.Search) (int64, error) {
	source := search.Source
	if source == "" {
		source = "yad2"
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO searches (chat_id, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		search.ChatID, search.Name, source, search.Manufacturer, search.Model,
		search.YearMin, search.YearMax, search.PriceMax,
		search.EngineMinCC, search.MaxKm, search.MaxHand)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) ListSearches(ctx context.Context, chatID int64) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, active, created_at
		FROM searches WHERE chat_id = ? ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearches(rows)
}

func (s *Store) GetSearch(ctx context.Context, id int64) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, active, created_at
		FROM searches WHERE id = ?`, id)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Active, &search.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &search, nil
}

func (s *Store) DeleteSearch(ctx context.Context, id int64, chatID int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM searches WHERE id = ? AND chat_id = ?", id, chatID)
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

func (s *Store) SetSearchActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE searches SET active = ? WHERE id = ?", active, id)
	return err
}

func (s *Store) ListAllActiveSearches(ctx context.Context) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.chat_id, s.name, s.source, s.manufacturer, s.model, s.year_min, s.year_max, s.price_max, s.engine_min_cc, s.max_km, s.max_hand, s.active, s.created_at
		FROM searches s
		JOIN users u ON s.chat_id = u.chat_id
		WHERE s.active = true AND u.active = true
		ORDER BY s.source, s.manufacturer, s.model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
		if err := rows.Scan(&s.ID, &s.ChatID, &s.Name, &s.Source, &s.Manufacturer, &s.Model,
			&s.YearMin, &s.YearMax, &s.PriceMax,
			&s.EngineMinCC, &s.MaxKm, &s.MaxHand,
			&s.Active, &s.CreatedAt); err != nil {
			return nil, err
		}
		searches = append(searches, s)
	}
	return searches, rows.Err()
}

// --- DedupStore (per-user) ---

func (s *Store) ClaimNew(ctx context.Context, token string, chatID int64, searchID int64) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO seen_listings (token, chat_id, search_id) VALUES (?, ?, ?)",
		token, chatID, searchID)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (s *Store) ReleaseClaim(ctx context.Context, token string, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM seen_listings WHERE token = ? AND chat_id = ?", token, chatID)
	return err
}

func (s *Store) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM seen_listings WHERE first_seen_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// --- NotificationQueue ---

func (s *Store) EnqueueNotification(ctx context.Context, recipient, searchName, payload string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO pending_notifications (recipient, search_name, payload) VALUES (?, ?, ?)",
		recipient, searchName, payload)
	return err
}

func (s *Store) PendingNotifications(ctx context.Context) ([]storage.PendingNotification, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, recipient, search_name, payload FROM pending_notifications ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []storage.PendingNotification
	for rows.Next() {
		var p storage.PendingNotification
		if err := rows.Scan(&p.ID, &p.Recipient, &p.SearchName, &p.Payload); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

func (s *Store) AckNotification(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pending_notifications WHERE id = ?", id)
	return err
}

func (s *Store) PruneNotifications(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM pending_notifications WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// --- PriceTracker ---

func (s *Store) RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT price FROM price_history WHERE token = ? ORDER BY observed_at DESC LIMIT 1", token)
	var prev int
	scanErr := row.Scan(&prev)

	_, err = s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO price_history (token, price) VALUES (?, ?)", token, price)
	if err != nil {
		return 0, false, err
	}

	if scanErr == sql.ErrNoRows {
		return 0, false, nil
	}
	if scanErr != nil {
		return 0, false, scanErr
	}

	if price != prev {
		return prev, true, nil
	}
	return prev, false, nil
}

// --- ListingStore ---

func (s *Store) SaveListing(ctx context.Context, r storage.ListingRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO listing_history
		(token, chat_id, search_name, manufacturer, model, year, price, km, hand, city, page_link, first_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token, chat_id) DO UPDATE SET
			price = excluded.price,
			km = excluded.km`,
		r.Token, r.ChatID, r.SearchName, r.Manufacturer, r.Model, r.Year, r.Price,
		r.Km, r.Hand, r.City, r.PageLink, r.FirstSeenAt)
	return err
}

func (s *Store) ListUserListings(ctx context.Context, chatID int64, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, first_seen_at
		FROM listing_history
		WHERE chat_id = ?
		ORDER BY first_seen_at DESC, token DESC
		LIMIT ? OFFSET ?`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var listings []storage.ListingRecord
	for rows.Next() {
		var l storage.ListingRecord
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.FirstSeenAt); err != nil {
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
		SELECT token, search_name, manufacturer, model, year, price, km, hand, city, page_link, first_seen_at
		FROM listing_history
		GROUP BY token
		ORDER BY first_seen_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var listings []storage.ListingRecord
	for rows.Next() {
		var l storage.ListingRecord
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.FirstSeenAt); err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

// --- DigestStore ---

func (s *Store) SetDigestMode(ctx context.Context, chatID int64, mode string, interval string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET digest_mode = ?, digest_interval = ? WHERE chat_id = ?",
		mode, interval, chatID)
	return err
}

func (s *Store) GetDigestMode(ctx context.Context, chatID int64) (string, string, error) {
	var mode, interval string
	err := s.db.QueryRowContext(ctx,
		"SELECT digest_mode, digest_interval FROM users WHERE chat_id = ?",
		chatID).Scan(&mode, &interval)
	if err == sql.ErrNoRows {
		return "instant", "6h", nil
	}
	return mode, interval, err
}

func (s *Store) AddDigestItem(ctx context.Context, chatID int64, payload string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO pending_digest (chat_id, listing_payload) VALUES (?, ?)",
		chatID, payload)
	return err
}

func (s *Store) FlushDigest(ctx context.Context, chatID int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx,
		"SELECT listing_payload FROM pending_digest WHERE chat_id = ? ORDER BY created_at", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payloads []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		payloads = append(payloads, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(payloads) > 0 {
		if _, err := tx.ExecContext(ctx, "DELETE FROM pending_digest WHERE chat_id = ?", chatID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE users SET digest_last_flushed = CURRENT_TIMESTAMP WHERE chat_id = ?", chatID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return payloads, nil
}

func (s *Store) PendingDigestUsers(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT chat_id FROM pending_digest")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, id)
	}
	return chatIDs, rows.Err()
}

func (s *Store) DigestLastFlushed(ctx context.Context, chatID int64) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx,
		"SELECT digest_last_flushed FROM users WHERE chat_id = ?", chatID).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
}

// --- CatalogStore ---

func (s *Store) SaveCatalogEntries(ctx context.Context, entries []storage.CatalogEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM catalog_cache"); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO catalog_cache (manufacturer_id, manufacturer_name, model_id, model_name) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, e.ManufacturerID, e.ManufacturerName, e.ModelID, e.ModelName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadCatalogEntries(ctx context.Context) ([]storage.CatalogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT manufacturer_id, manufacturer_name, model_id, model_name FROM catalog_cache ORDER BY manufacturer_name, model_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

// --- Close ---

func (s *Store) Close() error {
	return s.db.Close()
}
