package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}

	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply sqlite pragma: %w", err)
		}
	}

	s := &Store{db: db}

	if err := s.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ensureSchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 1,
			monthly_limit INTEGER,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS email_attempts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			provider_id TEXT,
			sender TEXT NOT NULL,
			subject TEXT NOT NULL,
			recipient_count INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error_message TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_email_attempts_project_id
			ON email_attempts(project_id);`,
		`CREATE INDEX IF NOT EXISTS idx_email_attempts_created_at
			ON email_attempts(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_email_attempts_provider_id
			ON email_attempts(provider_id);`,
		`CREATE TABLE IF NOT EXISTS email_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email_attempt_id INTEGER,
			provider TEXT NOT NULL,
			provider_event_id TEXT,
			event_type TEXT NOT NULL,
			occurred_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (email_attempt_id) REFERENCES email_attempts(id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_email_events_attempt
			ON email_events(email_attempt_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_email_events_provider_event
			ON email_events(provider, provider_event_id)
			WHERE provider_event_id IS NOT NULL AND provider_event_id <> '';`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("ensure sqlite schema: %w", err)
		}
	}

	return nil
}
