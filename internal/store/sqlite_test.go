package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSQLiteStore(t *testing.T) {
	ctx := context.Background()
	var s Store = NewSQLite(t.TempDir(), time.Minute)

	if err := s.Create(ctx, "BAD"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("create invalid: %v", err)
	}
	if _, err := s.Query(ctx, "missing", "SELECT 1", nil); !errors.Is(err, ErrNotFound) {
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
	if _, err := s.Query(ctx, "beta", "SELECT * FROM notes", nil); !errors.Is(err, ErrBadQuery) {
		t.Fatalf("beta sees alpha's table: %v", err)
	}
}
