// Package store gives each object its own database. The HTTP server talks to
// the Store interface; adapters (SQLite first) decide how state is persisted.
package store

import (
	"context"
	"errors"
	"regexp"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
)

// Store is the set of object operations the API needs.
type Store interface {
	// Create makes the object's empty database. Creating an existing object
	// leaves its data alone. A non-nil error is an *errs.Error.
	Create(ctx context.Context, id string) error
	// Query runs client SQL against an existing object in its own transaction.
	// SQLite's authorizer limits what it may do (see authorizeQuery).
	// A non-nil error is an *errs.Error. Bad statements and SQL errors are
	// errs.BadQuery.
	Query(ctx context.Context, id, query string, params []any) (QueryResult, error)
}

// Markers that wrapErr classifies at the boundary. Their text is the client
// Message.
var (
	errInvalidID = errors.New("invalid object ID")
	errNotFound  = errors.New("object not found")
	errBadQuery  = errors.New("bad query")
)

// QueryResult is the column names and rows a Query returned.
type QueryResult struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// Object IDs become directory names, so the store only accepts safe ones.
var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// wrapErr is where the store's errors cross its boundary as *errs.Error. The
// exported methods call it for every error they return. It returns nil for nil.
// (call in registry.go also makes an *errs.Error, but only to keep a panic's
// stack; wrapErr still picks the Kind and Message.)
func wrapErr(op, id string, err error) error {
	if err == nil {
		return nil
	}
	kind, msg := errs.Internal, "storage error"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		kind, msg = errs.Canceled, "request timed out"
	case errors.Is(err, context.Canceled):
		kind, msg = errs.Canceled, "request canceled"
	case errors.Is(err, errInvalidID):
		kind, msg = errs.Invalid, errInvalidID.Error()
	case errors.Is(err, errNotFound):
		kind, msg = errs.NotFound, errNotFound.Error()
	case errors.Is(err, errBadQuery):
		// The SQL came from the client, so the SQLite text is the message.
		kind, msg = errs.BadQuery, err.Error()
	}
	return errs.Wrap(err, kind, op, msg).With("object", id)
}
