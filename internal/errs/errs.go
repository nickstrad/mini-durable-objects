// Package errs is the error shape every package returns across a boundary.
// See CODING_STYLE.md for when to wrap an error and when to forward it.
package errs

import (
	"errors"
	"runtime/debug"
)

// Kind is the category of a failure. Callers use it to decide what to do.
type Kind int

const (
	Internal Kind = iota // a bug or unexpected failure; never show details
	Invalid              // the caller sent bad input: a bad ID or bad JSON
	NotFound             // the object does not exist
	BadQuery             // SQLite rejected the client's SQL, or the SQL failed
	Canceled             // the caller stopped the request, or a deadline passed
)

func (k Kind) String() string {
	switch k {
	case Invalid:
		return "invalid"
	case NotFound:
		return "not_found"
	case BadQuery:
		return "bad_query"
	case Canceled:
		return "canceled"
	default:
		return "internal"
	}
}

// Error is an error that crossed a boundary.
type Error struct {
	Kind    Kind
	Op      string         // the operation: "store.Query", "server.sql"
	Message string         // safe to show to the client
	Inner   error          // the cause; may hold internal details
	Stack   []byte         // Internal only; recorded once, at the first wrap
	Misc    map[string]any // debug data, e.g. {"object": id}
}

func (e *Error) Error() string {
	// Skip a cause that only repeats the Message, such as a package's own
	// marker error ("object not found: object not found").
	if e.Inner == nil || e.Inner.Error() == e.Message {
		return e.Op + ": " + e.Message
	}
	return e.Op + ": " + e.Message + ": " + e.Inner.Error()
}

func (e *Error) Unwrap() error { return e.Inner }

// Wrap makes err into an *Error at a boundary. err may be nil for a failure
// that has no underlying cause.
//
// Only Internal errors get a stack: nobody reads the stack of a client error
// such as BadQuery. If err already holds a stack, Wrap keeps it, because the
// first stack is the one nearest the cause.
func Wrap(err error, kind Kind, op, message string) *Error {
	e := &Error{Kind: kind, Op: op, Message: message, Inner: err}
	if inner, ok := errors.AsType[*Error](err); ok && inner.Stack != nil {
		e.Stack = inner.Stack
	} else if kind == Internal {
		e.Stack = debug.Stack()
	}
	return e
}

// With adds a debug value to e and returns e, for chaining after Wrap.
func (e *Error) With(key string, value any) *Error {
	if e.Misc == nil {
		e.Misc = map[string]any{}
	}
	e.Misc[key] = value
	return e
}

// KindOf reports err's Kind. An error that is not an *Error is Internal: it
// crossed a boundary without being wrapped, which is a bug.
func KindOf(err error) Kind {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Kind
	}
	return Internal
}
