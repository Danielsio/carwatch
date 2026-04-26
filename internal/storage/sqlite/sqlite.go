package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/dsionov/carwatch/internal/storage"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if dbPath != ":memory:" && !strings.HasPrefix(dbPath, "file::memory:") {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	db, err := sql.Open("sqlite3", dbPath+sep+"_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	if err := migrate(db); err != nil {
		_ = db.Close()
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

		CREATE TABLE IF NOT EXISTS saved_listings (
			chat_id   INTEGER NOT NULL,
			token     TEXT NOT NULL,
			saved_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (chat_id, token)
		);

		CREATE TABLE IF NOT EXISTS hidden_listings (
			chat_id    INTEGER NOT NULL,
			token      TEXT NOT NULL,
			hidden_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (chat_id, token)
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
			SELECT DISTINCT
				lh.token, COALESCE(sl.chat_id, 0), lh.search_name, lh.manufacturer, lh.model,
				lh.year, lh.price, lh.km, lh.hand, lh.city, lh.page_link, lh.first_seen_at
			FROM listing_history lh
			LEFT JOIN seen_listings sl ON sl.token = lh.token;
			DROP TABLE listing_history;
			ALTER TABLE listing_history_v2 RENAME TO listing_history;
		`); err != nil {
			return fmt.Errorf("migrate listing_history: %w", err)
		}
	}

	// Add language column to users table if it doesn't exist.
	var hasLanguage int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'language'",
	).Scan(&hasLanguage); err != nil {
		return fmt.Errorf("check users language: %w", err)
	}
	if hasLanguage == 0 {
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN language TEXT NOT NULL DEFAULT 'he'"); err != nil {
			return fmt.Errorf("add language column: %w", err)
		}
	}

	// Add keywords columns to searches table if they don't exist.
	for _, col := range []struct {
		name string
		def  string
	}{
		{"keywords", "TEXT NOT NULL DEFAULT ''"},
		{"exclude_keys", "TEXT NOT NULL DEFAULT ''"},
	} {
		var colCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('searches') WHERE name = ?", col.name,
		).Scan(&colCount)
		if err != nil {
			return fmt.Errorf("check column %s: %w", col.name, err)
		}
		if colCount == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE searches ADD COLUMN %s %s", col.name, col.def))
			if err != nil {
				return fmt.Errorf("add column %s: %w", col.name, err)
			}
		}
	}

	// Add user_seq column to searches table if it doesn't exist.
	var hasUserSeq int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('searches') WHERE name = 'user_seq'",
	).Scan(&hasUserSeq); err != nil {
		return fmt.Errorf("check searches user_seq: %w", err)
	}
	if hasUserSeq == 0 {
		if _, err := db.Exec("ALTER TABLE searches ADD COLUMN user_seq INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add user_seq column: %w", err)
		}
		// Backfill: assign sequential numbers per user based on creation order.
		if _, err := db.Exec(`
			UPDATE searches SET user_seq = (
				SELECT COUNT(*) FROM searches s2
				WHERE s2.chat_id = searches.chat_id AND s2.id <= searches.id
			)
		`); err != nil {
			return fmt.Errorf("backfill user_seq: %w", err)
		}
		if _, err := db.Exec(
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_searches_chat_user_seq ON searches(chat_id, user_seq)",
		); err != nil {
			return fmt.Errorf("create user_seq index: %w", err)
		}
	}

	// Add tier columns to users table if they don't exist.
	for _, col := range []struct {
		name string
		def  string
	}{
		{"tier", "TEXT NOT NULL DEFAULT 'free'"},
		{"tier_expires_at", "TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00'"},
		{"trial_used", "BOOLEAN NOT NULL DEFAULT false"},
	} {
		var tierCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = ?", col.name,
		).Scan(&tierCount)
		if err != nil {
			return fmt.Errorf("check column %s: %w", col.name, err)
		}
		if tierCount == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", col.name, col.def))
			if err != nil {
				return fmt.Errorf("add column %s: %w", col.name, err)
			}
		}
	}

	// Add daily digest columns to users table if they don't exist.
	for _, col := range []struct {
		name string
		def  string
	}{
		{"daily_digest", "BOOLEAN NOT NULL DEFAULT false"},
		{"daily_digest_time", "TEXT NOT NULL DEFAULT '09:00'"},
		{"daily_digest_last_sent", "TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00'"},
	} {
		var ddCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = ?", col.name,
		).Scan(&ddCount)
		if err != nil {
			return fmt.Errorf("check column %s: %w", col.name, err)
		}
		if ddCount == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", col.name, col.def))
			if err != nil {
				return fmt.Errorf("add column %s: %w", col.name, err)
			}
		}
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_listing_history_chat_first_seen
			ON listing_history(chat_id, first_seen_at DESC);
		CREATE INDEX IF NOT EXISTS idx_listing_history_token
			ON listing_history(token);
		CREATE INDEX IF NOT EXISTS idx_price_history_token_observed
			ON price_history(token, observed_at DESC)
	`); err != nil {
		return fmt.Errorf("create performance indexes: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS market_cache (
			token        TEXT PRIMARY KEY,
			manufacturer TEXT NOT NULL,
			model        TEXT NOT NULL,
			year         INTEGER NOT NULL,
			price        INTEGER NOT NULL,
			updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create market_cache: %w", err)
	}

	var cacheCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM market_cache").Scan(&cacheCount); err != nil {
		return fmt.Errorf("count market_cache: %w", err)
	}
	if cacheCount == 0 {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO market_cache (token, manufacturer, model, year, price)
			SELECT token, MAX(manufacturer), MAX(model), MAX(year), MAX(price)
			FROM listing_history
			WHERE manufacturer IS NOT NULL AND manufacturer != ''
			  AND model IS NOT NULL AND model != ''
			  AND year > 0 AND price > 0
			GROUP BY token
		`); err != nil {
			return fmt.Errorf("backfill market_cache: %w", err)
		}
	}

	for _, col := range []struct {
		name string
		def  string
	}{
		{"channel", "TEXT NOT NULL DEFAULT 'telegram'"},
		{"channel_id", "TEXT NOT NULL DEFAULT ''"},
	} {
		var chanCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = ?", col.name,
		).Scan(&chanCount)
		if err != nil {
			return fmt.Errorf("check column %s: %w", col.name, err)
		}
		if chanCount == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", col.name, col.def))
			if err != nil {
				return fmt.Errorf("add column %s: %w", col.name, err)
			}
		}
	}

	if _, err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_channel_id
			ON users(channel, channel_id) WHERE channel_id != ''
	`); err != nil {
		return fmt.Errorf("create channel_id index: %w", err)
	}

	return nil
}

const whatsappIDOffset int64 = 1_000_000_000_000

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
		"SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE chat_id = ?",
		chatID)

	var u storage.User
	err := row.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
		&u.Tier, &u.TierExpires, &u.TrialUsed, &u.Channel, &u.ChannelID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByChannelID(ctx context.Context, channel, channelID string) (*storage.User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE channel = ? AND channel_id = ?",
		channel, channelID)

	var u storage.User
	err := row.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
		&u.Tier, &u.TierExpires, &u.TrialUsed, &u.Channel, &u.ChannelID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpsertWhatsAppUser(ctx context.Context, phoneNumber string) (int64, error) {
	existing, err := s.GetUserByChannelID(ctx, "whatsapp", phoneNumber)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.ChatID, nil
	}

	var maxID int64
	_ = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(chat_id), ?) FROM users WHERE chat_id >= ?",
		whatsappIDOffset-1, whatsappIDOffset).Scan(&maxID)
	newID := maxID + 1

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO users (chat_id, username, channel, channel_id) VALUES (?, ?, 'whatsapp', ?)",
		newID, phoneNumber, phoneNumber)
	if err != nil {
		return 0, fmt.Errorf("create whatsapp user: %w", err)
	}
	return newID, nil
}

func (s *Store) UpdateUserState(ctx context.Context, chatID int64, state string, stateData string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET state = ?, state_data = ? WHERE chat_id = ?",
		state, stateData, chatID)
	return err
}

func (s *Store) ListActiveUsers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used FROM users WHERE active = true")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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

func (s *Store) SetUserLanguage(ctx context.Context, chatID int64, lang string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET language = ? WHERE chat_id = ?",
		lang, chatID)
	return err
}

func scanUsers(rows *sql.Rows) ([]storage.User, error) {
	var users []storage.User
	for rows.Next() {
		var u storage.User
		if err := rows.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
			&u.Tier, &u.TierExpires, &u.TrialUsed); err != nil {
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

	result, err := tx.ExecContext(ctx, `
		INSERT INTO searches (chat_id, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, user_seq)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		search.ChatID, search.Name, source, search.Manufacturer, search.Model,
		search.YearMin, search.YearMax, search.PriceMax,
		search.EngineMinCC, search.MaxKm, search.MaxHand,
		search.Keywords, search.ExcludeKeys, nextSeq)
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
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at
		FROM searches WHERE chat_id = ? ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSearches(rows)
}

func (s *Store) GetSearch(ctx context.Context, id int64) (*storage.Search, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at
		FROM searches WHERE id = ?`, id)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys,
		&search.Active, &search.CreatedAt)
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
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max, price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at
		FROM searches WHERE chat_id = ? AND user_seq = ?`, chatID, seq)

	var search storage.Search
	err := row.Scan(&search.ID, &search.ChatID, &search.UserSeq, &search.Name, &search.Source, &search.Manufacturer, &search.Model,
		&search.YearMin, &search.YearMax, &search.PriceMax,
		&search.EngineMinCC, &search.MaxKm, &search.MaxHand,
		&search.Keywords, &search.ExcludeKeys,
		&search.Active, &search.CreatedAt)
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

func (s *Store) SetSearchActive(ctx context.Context, id int64, chatID int64, active bool) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE searches SET active = ? WHERE id = ? AND chat_id = ?", active, id, chatID)
	return err
}

func (s *Store) ListAllActiveSearches(ctx context.Context) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.chat_id, s.user_seq, s.name, s.source, s.manufacturer, s.model, s.year_min, s.year_max, s.price_max, s.engine_min_cc, s.max_km, s.max_hand, s.keywords, s.exclude_keys, s.active, s.created_at
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
	defer func() { _ = rows.Close() }()

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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		"SELECT price FROM price_history WHERE token = ? ORDER BY observed_at DESC, rowid DESC LIMIT 1", token)
	var prev int
	scanErr := row.Scan(&prev)

	_, err = tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO price_history (token, price) VALUES (?, ?)", token, price)
	if err != nil {
		return 0, false, err
	}

	if scanErr == sql.ErrNoRows {
		return 0, false, tx.Commit()
	}
	if scanErr != nil {
		return 0, false, scanErr
	}

	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	if price != prev {
		return prev, true, nil
	}
	return prev, false, nil
}

func (s *Store) PrunePrices(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM price_history WHERE observed_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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


func (s *Store) PeekDigest(ctx context.Context, chatID int64) ([]string, time.Time, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT listing_payload FROM pending_digest WHERE chat_id = ? ORDER BY created_at", chatID)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer func() { _ = rows.Close() }()

	cutoff := time.Now()
	var payloads []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, time.Time{}, err
		}
		payloads = append(payloads, p)
	}
	return payloads, cutoff, rows.Err()
}

func (s *Store) AckDigest(ctx context.Context, chatID int64, before time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM pending_digest WHERE chat_id = ? AND created_at <= ?", chatID, before); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE users SET digest_last_flushed = CURRENT_TIMESTAMP WHERE chat_id = ?", chatID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PendingDigestUsers(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT chat_id FROM pending_digest")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

// --- SavedListingStore ---

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
			lh.km, lh.hand, lh.city, lh.page_link, lh.first_seen_at
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
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.FirstSeenAt); err != nil {
			return nil, err
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

// --- HiddenListingStore ---

func (s *Store) HideListing(ctx context.Context, chatID int64, token string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO hidden_listings (chat_id, token) VALUES (?, ?)",
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

// --- MarketStore ---

func (s *Store) MarketListings(ctx context.Context) ([]storage.MarketListing, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT manufacturer, model, year, price
		FROM market_cache
		WHERE manufacturer != '' AND model != ''
		  AND year > 0 AND price > 0`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.MarketListing
	for rows.Next() {
		var l storage.MarketListing
		if err := rows.Scan(&l.Manufacturer, &l.Model, &l.Year, &l.Price); err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

// --- DailyDigestStore ---

func (s *Store) SetDailyDigest(ctx context.Context, chatID int64, enabled bool, digestTime string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET daily_digest = ?, daily_digest_time = ? WHERE chat_id = ?",
		enabled, digestTime, chatID)
	return err
}

func (s *Store) GetDailyDigest(ctx context.Context, chatID int64) (bool, string, time.Time, error) {
	var enabled bool
	var digestTime string
	var lastSent time.Time
	err := s.db.QueryRowContext(ctx,
		"SELECT daily_digest, daily_digest_time, daily_digest_last_sent FROM users WHERE chat_id = ?",
		chatID).Scan(&enabled, &digestTime, &lastSent)
	if err == sql.ErrNoRows {
		return false, "09:00", time.Time{}, nil
	}
	return enabled, digestTime, lastSent, err
}

func (s *Store) UpdateDailyDigestLastSent(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET daily_digest_last_sent = ? WHERE chat_id = ?",
		time.Now(), chatID)
	return err
}

func (s *Store) ListDailyDigestUsers(ctx context.Context) ([]storage.DailyDigestUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, daily_digest_time, daily_digest_last_sent
		FROM users
		WHERE daily_digest = true AND active = true`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []storage.DailyDigestUser
	for rows.Next() {
		var u storage.DailyDigestUser
		if err := rows.Scan(&u.ChatID, &u.DigestTime, &u.LastSent); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) DailyStats(ctx context.Context, chatID int64) ([]storage.DailySearchStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH search_listings AS (
			SELECT s.id AS search_id, s.name AS search_name,
			       lh.price, lh.page_link, lh.first_seen_at
			FROM listing_history lh
			JOIN searches s ON s.name = lh.search_name AND s.chat_id = lh.chat_id
			WHERE lh.chat_id = ? AND s.active = true AND lh.price > 0
		)
		SELECT
			sl.search_name,
			SUM(CASE WHEN sl.first_seen_at >= datetime('now', '-1 day') THEN 1 ELSE 0 END),
			CAST(AVG(sl.price) AS INTEGER),
			MIN(sl.price),
			(SELECT sl2.page_link FROM search_listings sl2
			 WHERE sl2.search_id = sl.search_id
			 ORDER BY sl2.price ASC LIMIT 1),
			AVG(CASE WHEN sl.first_seen_at >= datetime('now', '-1 day') THEN sl.price END),
			AVG(CASE WHEN sl.first_seen_at >= datetime('now', '-7 days')
			          AND sl.first_seen_at < datetime('now', '-1 day') THEN sl.price END)
		FROM search_listings sl
		GROUP BY sl.search_id
		HAVING COUNT(*) >= 5`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stats []storage.DailySearchStats
	for rows.Next() {
		var ds storage.DailySearchStats
		var recentAvg, olderAvg sql.NullFloat64
		if err := rows.Scan(&ds.SearchName, &ds.NewCount, &ds.AvgPrice,
			&ds.BestPrice, &ds.BestPriceLink, &recentAvg, &olderAvg); err != nil {
			return nil, err
		}
		if recentAvg.Valid && olderAvg.Valid && olderAvg.Float64 > 0 {
			ds.PriceTrend = ((recentAvg.Float64 - olderAvg.Float64) / olderAvg.Float64) * 100
		}
		stats = append(stats, ds)
	}
	return stats, rows.Err()
}

// --- TierStore ---

func (s *Store) SetUserTier(ctx context.Context, chatID int64, tier string, expires time.Time) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET tier = ?, tier_expires_at = ? WHERE chat_id = ?",
		tier, expires, chatID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) GrantTrial(ctx context.Context, chatID int64, duration time.Duration) error {
	expires := time.Now().Add(duration)
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET tier = 'premium', tier_expires_at = ?, trial_used = true WHERE chat_id = ? AND trial_used = false",
		expires, chatID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) ListExpiredPremium(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used
		FROM users
		WHERE tier = 'premium' AND tier_expires_at <= ? AND tier_expires_at > '1970-01-01 00:00:00'`,
		time.Now())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsers(rows)
}

// --- Close ---

func (s *Store) Close() error {
	return s.db.Close()
}
