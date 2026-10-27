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
// Each open database is owned by its object's goroutine until it sits idle.
type SQLite struct {
	dir     string
	objects *registry[*sql.DB]
}

var _ Store = SQLite{}

const idleTimeout = 3 * time.Second

func NewSQLite(dir string) SQLite {
	return SQLite{dir: dir, objects: newRegistry(idleTimeout, openDB)}
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

// do runs fn with an existing object's database on the object's goroutine.
func (s SQLite) do(ctx context.Context, id string, fn func(*sql.DB) error) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return s.objects.do(ctx, id, path, fn)
}

func (s SQLite) Create(ctx context.Context, id string) (string, error) {
	path, err := s.path(id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	var status string
	err = s.objects.do(ctx, id, path, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `
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
			return err
		}
		status, err = readStatus(ctx, db)
		return err
	})
	return status, err
}

func readStatus(ctx context.Context, db *sql.DB) (string, error) {
	var status string
	err := db.QueryRowContext(ctx, "SELECT status FROM task WHERE id=1").Scan(&status)
	return status, err
}

func (s SQLite) State(ctx context.Context, id string) (string, error) {
	var status string
	err := s.do(ctx, id, func(db *sql.DB) error {
		var err error
		status, err = readStatus(ctx, db)
		return err
	})
	return status, err
}

func (s SQLite) Claim(ctx context.Context, id string) (bool, error) {
	var claimed bool
	err := s.do(ctx, id, func(db *sql.DB) error {
		// Pin BEGIN, UPDATE, and COMMIT to one SQLite connection.
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout=2000"); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return err
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
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return err
		}
		committed = true
		claimed = affected == 1
		return nil
	})
	return claimed, err
}

func (s SQLite) Finish(ctx context.Context, id string, failBeforeCommit bool) error {
	return s.do(ctx, id, func(db *sql.DB) error {
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
	})
}
