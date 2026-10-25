package store

import (
	"context"
	"errors"
	"testing"
)

func TestSQLiteStore(t *testing.T) {
	ctx := context.Background()
	var s Store = NewSQLite(t.TempDir())

	if _, err := s.Create(ctx, "BAD"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("create invalid: %v", err)
	}
	if _, err := s.State(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("state missing: %v", err)
	}
	if status, err := s.Create(ctx, "task-alpha"); err != nil || status != "pending" {
		t.Fatalf("create: %q %v", status, err)
	}
	if err := s.Finish(ctx, "task-alpha", false); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("finish pending: %v", err)
	}
	if claimed, err := s.Claim(ctx, "task-alpha"); err != nil || !claimed {
		t.Fatalf("claim: %v %v", claimed, err)
	}
	if claimed, err := s.Claim(ctx, "task-alpha"); err != nil || claimed {
		t.Fatalf("repeat claim: %v %v", claimed, err)
	}
	if status, err := s.Create(ctx, "task-alpha"); err != nil || status != "running" {
		t.Fatalf("create existing: %q %v", status, err)
	}
	if err := s.Finish(ctx, "task-alpha", true); !errors.Is(err, ErrSimulatedFailure) {
		t.Fatalf("failed finish: %v", err)
	}
	if status, _ := s.State(ctx, "task-alpha"); status != "running" {
		t.Fatalf("after rollback: %q", status)
	}
	if err := s.Finish(ctx, "task-alpha", false); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if status, _ := s.State(ctx, "task-alpha"); status != "completed" {
		t.Fatalf("after finish: %q", status)
	}
}
