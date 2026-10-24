package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLite stores each object in its own database at <Dir>/<id>/state.sqlite.
type SQLite struct{ Dir string }

var _ Store = SQLite{}

// Lesson 1 path convention: each object ID maps to its own directory.
func (s SQLite) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", ErrInvalidID
	}
	return filepath.Join(s.Dir, id, "state.sqlite"), nil
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// open opens an existing object's database.
func (s SQLite) open(id string) (*sql.DB, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return openDB(path)
}

func (s SQLite) Create(ctx context.Context, id string) (string, error) {
	path, err := s.path(id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	db, err := openDB(path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS task (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			status TEXT NOT NULL DEFAULT 'pending'
		);
		INSERT OR IGNORE INTO task(id) VALUES(1);
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY,
			kind TEXT NOT NULL
		)
	`)
	if err != nil {
		return "", err
	}
	return readStatus(ctx, db)
}

func readStatus(ctx context.Context, db *sql.DB) (string, error) {
	var status string
	err := db.QueryRowContext(ctx, "SELECT status FROM task WHERE id=1").Scan(&status)
	return status, err
}

func (s SQLite) State(ctx context.Context, id string) (string, error) {
	db, err := s.open(id)
	if err != nil {
		return "", err
	}
	defer db.Close()
	return readStatus(ctx, db)
}

func (s SQLite) Claim(ctx context.Context, id string) (bool, error) {
	db, err := s.open(id)
	if err != nil {
		return false, err
	}
	defer db.Close()
	// Pin BEGIN, UPDATE, and COMMIT to one SQLite connection.
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout=2000"); err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			// Rollback must still run if the request was canceled.
			conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	result, err := conn.ExecContext(ctx, "UPDATE task SET status='running' WHERE id=1 AND status='pending'")
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, err
	}
	committed = true
	return affected == 1, nil
}

func (s SQLite) Finish(ctx context.Context, id string, failBeforeCommit bool) error {
	db, err := s.open(id)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE task SET status='completed' WHERE id=1 AND status='running'")
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotRunning
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO events(kind) VALUES('completed')"); err != nil {
		return err
	}
	if failBeforeCommit {
		return ErrSimulatedFailure
	}
	return tx.Commit()
}
