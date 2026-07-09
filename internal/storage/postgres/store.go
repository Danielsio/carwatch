package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

func New(dsn string, migrationsPath string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if migrationsPath != "" {
		absPath, err := filepath.Abs(migrationsPath)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("resolve migrations path: %w", err)
		}
		driver, err := pgx.WithInstance(db, &pgx.Config{})
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create migrate driver: %w", err)
		}
		// Use the iofs source over os.DirFS rather than a "file://" URL: the
		// file source's URL parsing is broken on Windows (a `C:\…` path parses
		// as an invalid port, and even when slash-normalized the source can't
		// open it), which breaks `make dev` on Windows. iofs takes an fs.FS
		// directly — no URL, no platform-specific path munging — and still
		// reads migrations from disk at runtime (prod mounts ./migrations).
		src, err := iofs.New(os.DirFS(absPath), ".")
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("open migrations source: %w", err)
		}
		m, err := migrate.NewWithInstance("iofs", src, "pgx", driver)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create migrator: %w", err)
		}
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			_ = db.Close()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DBSizeBytes() (int64, error) {
	var size int64
	err := s.db.QueryRow("SELECT pg_database_size(current_database())").Scan(&size)
	return size, err
}
