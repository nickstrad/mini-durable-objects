// Package store gives each object its own database. The HTTP server talks to
// the Store interface; adapters (SQLite first) decide how state is persisted.
package store

import (
	"context"
	"errors"
	"regexp"
)

// Store is the set of object operations the API needs.
type Store interface {
	// Create makes the object's empty database. Creating an existing object
	// leaves its data alone.
	Create(ctx context.Context, id string) error
	// Query runs client SQL against an existing object in its own transaction.
	// SQLite's authorizer limits what it may do (see authorizeQuery).
	// Bad statements and SQL errors return ErrBadQuery.
	Query(ctx context.Context, id, query string, params []any) (QueryResult, error)
}

var (
	ErrInvalidID = errors.New("invalid object ID")
	ErrNotFound  = errors.New("object not found")
	ErrBadQuery  = errors.New("bad query")
)

// QueryResult is the column names and rows a Query returned.
type QueryResult struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// Object IDs become directory names, so the store only accepts safe ones.
var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
