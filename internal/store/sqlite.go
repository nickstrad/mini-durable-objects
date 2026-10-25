package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite stores each object in its own database at <dir>/<id>/state.sqlite.
// Open databases stay in a registry until they sit idle.
type SQLite struct {
	dir     string
	objects *registry
}

var _ Store = SQLite{}

const idleTimeout = 3 * time.Second

func NewSQLite(dir string) SQLite {
	return SQLite{dir: dir, objects: newRegistry(idleTimeout)}
}

// Lesson 1 path convention: each object ID maps to its own directory.
func (s SQLite) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", ErrInvalidID
	}
	return filepath.Join(s.dir, id, "state.sqlite"), nil
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// open returns an existing object's database. Call release when done.
func (s SQLite) open(id string) (db *sql.DB, release func(), err error) {
	path, err := s.path(id)
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNotFound
	} else if err != nil {
		return nil, nil, err
	}
	return s.objects.acquire(id, path)
}

func (s SQLite) Create(ctx context.Context, id string) (string, error) {
	path, err := s.path(id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	db, release, err := s.objects.acquire(id, path)
	if err != nil {
		return "", err
	}
	defer release()
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
	db, release, err := s.open(id)
	if err != nil {
		return "", err
	}
	defer release()
	return readStatus(ctx, db)
}

func (s SQLite) Claim(ctx context.Context, id string) (bool, error) {
	db, release, err := s.open(id)
	if err != nil {
		return false, err
	}
	defer release()
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
	db, release, err := s.open(id)
	if err != nil {
		return err
	}
	defer release()
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
