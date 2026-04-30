package sqlite

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// generateShareToken returns a 32-character hex string from 16 random bytes.
func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
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

		CREATE INDEX IF NOT EXISTS idx_pending_digest_chat_created
			ON pending_digest(chat_id, created_at);

		CREATE INDEX IF NOT EXISTS idx_searches_active
			ON searches(active, chat_id);

		CREATE INDEX IF NOT EXISTS idx_pending_notifications_created
			ON pending_notifications(created_at);

		CREATE INDEX IF NOT EXISTS idx_listing_history_chat_search
			ON listing_history(chat_id, search_name);
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

	if _, err := db.Exec(`
		UPDATE users SET channel_id = CAST(chat_id AS TEXT)
		WHERE channel = 'telegram' AND channel_id = ''
	`); err != nil {
		return fmt.Errorf("backfill telegram channel_id: %w", err)
	}

	// Add share_token column to searches table if it doesn't exist.
	var hasShareToken int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('searches') WHERE name = 'share_token'",
	).Scan(&hasShareToken); err != nil {
		return fmt.Errorf("check searches share_token: %w", err)
	}
	if hasShareToken == 0 {
		if _, err := db.Exec("ALTER TABLE searches ADD COLUMN share_token TEXT"); err != nil {
			return fmt.Errorf("add share_token column: %w", err)
		}
		if _, err := db.Exec(
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_searches_share_token ON searches(share_token) WHERE share_token IS NOT NULL",
		); err != nil {
			return fmt.Errorf("create share_token index: %w", err)
		}
		// Backfill existing rows with random tokens.
		rows, err := db.Query("SELECT id FROM searches WHERE share_token IS NULL")
		if err != nil {
			return fmt.Errorf("query searches for backfill: %w", err)
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan search id: %w", err)
			}
			ids = append(ids, id)
		}
		_ = rows.Close()
		for _, id := range ids {
			token, err := generateShareToken()
			if err != nil {
				return fmt.Errorf("generate share token: %w", err)
			}
			if _, err := db.Exec("UPDATE searches SET share_token = ? WHERE id = ?", token, id); err != nil {
				return fmt.Errorf("backfill share_token for id %d: %w", id, err)
			}
		}
	}

	// Add fitness_score column to listing_history if it doesn't exist.
	var hasFitness int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('listing_history') WHERE name = 'fitness_score'",
	).Scan(&hasFitness); err != nil {
		return fmt.Errorf("check listing_history fitness_score: %w", err)
	}
	if hasFitness == 0 {
		if _, err := db.Exec("ALTER TABLE listing_history ADD COLUMN fitness_score REAL"); err != nil {
			return fmt.Errorf("add fitness_score column: %w", err)
		}
	}

	// Add image_url column to listing_history if it doesn't exist.
	var hasImageURL int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('listing_history') WHERE name = 'image_url'",
	).Scan(&hasImageURL); err != nil {
		return fmt.Errorf("check listing_history image_url: %w", err)
	}
	if hasImageURL == 0 {
		if _, err := db.Exec("ALTER TABLE listing_history ADD COLUMN image_url TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("add image_url column: %w", err)
		}
	}

	// Add last_seen_at column to users table if it doesn't exist.
	var hasLastSeen int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'last_seen_at'",
	).Scan(&hasLastSeen); err != nil {
		return fmt.Errorf("check users last_seen_at: %w", err)
	}
	if hasLastSeen == 0 {
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN last_seen_at TIMESTAMP"); err != nil {
			return fmt.Errorf("add last_seen_at column: %w", err)
		}
	}

	// Add search_id column to listing_history for per-search filtering.
	var hasSearchID int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('listing_history') WHERE name = 'search_id'",
	).Scan(&hasSearchID); err != nil {
		return fmt.Errorf("check listing_history search_id: %w", err)
	}
	if hasSearchID == 0 {
		if _, err := db.Exec("ALTER TABLE listing_history ADD COLUMN search_id INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add search_id column: %w", err)
		}
		// Backfill search_id from searches table where names match.
		if _, err := db.Exec(`
			UPDATE listing_history SET search_id = (
				SELECT s.id FROM searches s
				WHERE s.chat_id = listing_history.chat_id
				  AND s.name = listing_history.search_name
				LIMIT 1
			) WHERE search_id = 0
		`); err != nil {
			return fmt.Errorf("backfill listing_history search_id: %w", err)
		}
		if _, err := db.Exec(`
			CREATE INDEX IF NOT EXISTS idx_listing_history_chat_searchid
				ON listing_history(chat_id, search_id)
		`); err != nil {
			return fmt.Errorf("create listing_history search_id index: %w", err)
		}
	}

	return nil
}
