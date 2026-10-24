// Package store keeps each object's task state. The HTTP server talks to the
// Store interface; adapters (SQLite first) decide how state is persisted.
package store

import (
	"context"
	"errors"
	"regexp"
)

// Store is the set of task operations the API needs. Status is one of
// "pending", "running", or "completed".
type Store interface {
	// Create makes the object with a pending task. An existing object keeps its status.
	Create(ctx context.Context, id string) (status string, err error)
	// State reads the task status of an existing object.
	State(ctx context.Context, id string) (status string, err error)
	// Claim moves a pending task to running. Only one concurrent caller gets true.
	Claim(ctx context.Context, id string) (claimed bool, err error)
	// Finish completes a running task and records its event, atomically.
	// failBeforeCommit rolls both changes back to demonstrate atomicity.
	Finish(ctx context.Context, id string, failBeforeCommit bool) error
}

var (
	ErrInvalidID        = errors.New("invalid object ID")
	ErrNotFound         = errors.New("object not found")
	ErrNotRunning       = errors.New("task is not running")
	ErrSimulatedFailure = errors.New("simulated failure before commit")
)

// Object IDs become directory names, so the store only accepts safe ones.
var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
