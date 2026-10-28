package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SQLite stores each object in its own database at <dir>/<id>/state.sqlite.
// Each open database is owned by its object's goroutine until it sits idle.
type SQLite struct {
	dir     string
	objects *registry[*objectDB]
}

var _ Store = SQLite{}

// NewSQLite stores objects under dir. An object unused for idleTimeout has its
// database closed; the next request reopens it.
func NewSQLite(dir string, idleTimeout time.Duration) SQLite {
	return SQLite{dir: dir, objects: newRegistry(idleTimeout, openDB)}
}

// path maps each object ID to its own directory.
func (s SQLite) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errInvalidID
	}
	return filepath.Join(s.dir, id, "state.sqlite"), nil
}

// do runs fn with an existing object's database on the object's goroutine.
func (s SQLite) do(ctx context.Context, id string, fn func(*objectDB) error) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return errNotFound
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return s.objects.do(ctx, id, path, fn)
}

func (s SQLite) Create(ctx context.Context, id string) error {
	return wrapErr("store.Create", id, s.create(ctx, id))
}

func (s SQLite) create(ctx context.Context, id string) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	// Connecting creates the database file if it does not exist yet.
	return s.objects.do(ctx, id, path, func(db *objectDB) error {
		return db.PingContext(ctx)
	})
}
