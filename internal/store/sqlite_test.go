package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
)

func TestSQLiteStore(t *testing.T) {
	ctx := context.Background()
	var s Store = NewSQLite(t.TempDir(), time.Minute)

	if err := s.Create(ctx, "BAD"); errs.KindOf(err) != errs.Invalid {
		t.Fatalf("create invalid: %v", err)
	}
	if _, err := s.Query(ctx, "missing", "SELECT 1", nil); errs.KindOf(err) != errs.NotFound {
		t.Fatalf("query missing: %v", err)
	}
	if err := s.Create(ctx, "alpha"); err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, q := range []string{
		"CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL)",
		"INSERT INTO notes(body) VALUES('hello')",
	} {
		if _, err := s.Query(ctx, "alpha", q, nil); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// Creating an existing object keeps its data.
	if err := s.Create(ctx, "alpha"); err != nil {
		t.Fatalf("create existing: %v", err)
	}
	got, err := s.Query(ctx, "alpha", "SELECT id, body FROM notes", nil)
	want := QueryResult{Columns: []string{"id", "body"}, Rows: [][]any{{int64(1), "hello"}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("select: %+v %v", got, err)
	}
	// Objects don't share databases.
	if err := s.Create(ctx, "beta"); err != nil {
		t.Fatalf("create beta: %v", err)
	}
	if _, err := s.Query(ctx, "beta", "SELECT * FROM notes", nil); errs.KindOf(err) != errs.BadQuery {
		t.Fatalf("beta sees alpha's table: %v", err)
	}
}

// wrapped returns err as an *errs.Error, or fails the test.
func wrapped(t *testing.T, err error) *errs.Error {
	t.Helper()
	e, ok := errors.AsType[*errs.Error](err)
	if !ok {
		t.Fatalf("got %T %v, want *errs.Error", err, err)
	}
	return e
}

// An error from os crosses the boundary as an *errs.Error with a stack.
func TestSQLiteForeignErrorIsWrapped(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := NewSQLite(file, time.Minute) // A file cannot hold object directories.

	e := wrapped(t, s.Create(context.Background(), "alpha"))
	if e.Kind != errs.Internal || e.Message != "storage error" || e.Op != "store.Create" {
		t.Fatalf("got kind %v, message %q, op %q", e.Kind, e.Message, e.Op)
	}
	if len(e.Stack) == 0 || e.Misc["object"] != "alpha" {
		t.Fatalf("stack %d bytes, misc %v", len(e.Stack), e.Misc)
	}
}

func TestSQLiteCanceledRequest(t *testing.T) {
	s := NewSQLite(t.TempDir(), time.Minute)
	if err := s.Create(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Query(ctx, "alpha", "SELECT 1", nil)
	if e := wrapped(t, err); e.Kind != errs.Canceled || e.Message != "request canceled" {
		t.Fatalf("canceled: got %v", e)
	}
	ctx, cancel = context.WithTimeout(context.Background(), -time.Second)
	defer cancel()
	if e := wrapped(t, s.Create(ctx, "alpha")); e.Kind != errs.Canceled || e.Message != "request timed out" {
		t.Fatalf("deadline: got %v", e)
	}
}

func TestSQLiteBadQueryMessageIsSQLiteText(t *testing.T) {
	s := NewSQLite(t.TempDir(), time.Minute)
	ctx := context.Background()
	if err := s.Create(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
	_, err := s.Query(ctx, "alpha", "SELECT * FROM nope", nil)
	if e := wrapped(t, err); e.Kind != errs.BadQuery || e.Message != "bad query: no such table: nope" {
		t.Fatalf("got %v", e)
	}
	// The cause repeats the Message, so Error() shows it once.
	if got := err.Error(); got != "store.Query: bad query: no such table: nope" {
		t.Fatalf("Error() = %q", got)
	}
}
