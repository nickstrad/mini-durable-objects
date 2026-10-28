package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
	"github.com/nickstrad/mini-durable-objects/internal/events"
)

// SQLite stores each object in its own database at <dir>/<id>/state.sqlite.
// Each open database is owned by its object's goroutine until it sits idle.
type SQLite struct {
	dir     string
	objects *registry[*objectDB]
}

var _ Store = SQLite{}

// NewSQLite stores objects under dir. An object unused for idleTimeout has its
// database closed; the next request reopens it. rec may be nil.
func NewSQLite(dir string, idleTimeout time.Duration, logger *slog.Logger, rec *events.Recorder) SQLite {
	return SQLite{dir: dir, objects: newRegistry(idleTimeout, openDB, logger, rec)}
}

// path maps each object ID to its own directory.
func (s SQLite) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errInvalidID
	}
	return filepath.Join(s.dir, id, "state.sqlite"), nil
}

// do runs fn with an existing object's database on the object's goroutine.
func (s SQLite) do(ctx context.Context, id string, fn func(*objectDB) error, tm *timing) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return errNotFound
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return s.objects.doTimed(ctx, id, path, fn, tm)
}

func (s SQLite) Create(ctx context.Context, id string) error {
	start := time.Now()
	var tm timing
	err := wrapErr("store.Create", id, s.create(ctx, id, &tm))
	s.emit(start, id, "create", err, tm, 0)
	return err
}

// emit records one request event. It runs after wrapErr, so kind is the
// Kind the client sees.
func (s SQLite) emit(start time.Time, id, op string, err error, tm timing, rows int) {
	kind := "ok"
	if err != nil {
		kind, rows = errs.KindOf(err).String(), 0
	}
	s.objects.rec.Request(events.Request{Start: start, Object: id, Op: op, Kind: kind, Ran: tm.Ran, Depth: tm.Depth,
		Lock: tm.Lock, Open: tm.Open, Queue: tm.Queue, ExecAt: tm.ExecAt, Exec: tm.Exec, Total: time.Since(start), Rows: rows})
}

func (s SQLite) create(ctx context.Context, id string, tm *timing) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	// Connecting creates the database file if it does not exist yet.
	return s.objects.doTimed(ctx, id, path, func(db *objectDB) error {
		return db.PingContext(ctx)
	}, tm)
}
