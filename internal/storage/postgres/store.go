package postgres

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db  *sql.DB
	dsn string
}

func New(dsn string, migrationsPath string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

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
		m, err := migrate.NewWithDatabaseInstance("file://"+absPath, "pgx", driver)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create migrator: %w", err)
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			_ = db.Close()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	return &Store{db: db, dsn: dsn}, nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DBSizeBytes() (int64, error) {
	var size int64
	err := s.db.QueryRow("SELECT pg_database_size(current_database())").Scan(&size)
	return size, err
}
