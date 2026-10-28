package errs

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestWrap(t *testing.T) {
	first := Wrap(io.EOF, Internal, "store.Query", "storage error").With("object", "a")
	if len(first.Stack) == 0 {
		t.Fatal("first Internal wrap recorded no stack")
	}
	if !errors.Is(first, io.EOF) {
		t.Fatal("wrap lost the cause")
	}
	if got := first.Error(); got != "store.Query: storage error: EOF" {
		t.Fatalf("Error() = %q", got)
	}
	if first.Misc["object"] != "a" {
		t.Fatalf("Misc = %v", first.Misc)
	}

	second := Wrap(first, Internal, "server.sql", "try again")
	if !bytes.Equal(second.Stack, first.Stack) {
		t.Fatal("second wrap replaced the first stack")
	}
	if KindOf(second) != Internal || second.Message != "try again" {
		t.Fatalf("second = %v", second)
	}
}

func TestWrapStackOnlyForInternal(t *testing.T) {
	client := Wrap(io.EOF, BadQuery, "store.Query", "bad query")
	if client.Stack != nil {
		t.Fatal("a client error recorded a stack")
	}
	// Making a client error Internal records the stack then.
	if Wrap(client, Internal, "server.sql", "storage error").Stack == nil {
		t.Fatal("upgrade to Internal recorded no stack")
	}
}

func TestKindOf(t *testing.T) {
	if KindOf(errors.New("plain")) != Internal {
		t.Fatal("a plain error must be Internal")
	}
	if KindOf(Wrap(nil, NotFound, "op", "missing")) != NotFound {
		t.Fatal("KindOf lost the Kind")
	}
	if got := Wrap(nil, NotFound, "op", "missing").Error(); got != "op: missing" {
		t.Fatalf("Error() with no cause = %q", got)
	}
	if got := Wrap(errors.New("missing"), NotFound, "op", "missing").Error(); got != "op: missing" {
		t.Fatalf("Error() repeats a cause equal to Message: %q", got)
	}
}
