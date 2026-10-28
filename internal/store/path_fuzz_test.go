package store

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FuzzObjectPath checks the store's security boundary: an object ID becomes a
// directory name, so path must never let an ID escape the store directory.
func FuzzObjectPath(f *testing.F) {
	seeds := []string{
		"a", "obj-1", "0", "a-b-c",
		strings.Repeat("a", 40), strings.Repeat("a", 41),
		"", "..", ".", "../x", "x/..", "a/b", "/a", `a\b`, "A", "aB", "-a", "a\x00", "a\n", "\na",
		"a b", "a.b", "a_b",
		"%2e%2e", "a%2fb",
		"аbc",  // Cyrillic a
		"a․b",  // one dot leader
		"ａbc",  // fullwidth a
		"‥",    // two dot leader
		"a​b",  // zero width space
		"café", // non-ASCII letter
	}
	for _, s := range seeds {
		f.Add(s)
	}

	dir := filepath.Join(f.TempDir(), "objects")
	s := NewSQLite(dir, time.Minute, discard, nil)

	f.Fuzz(func(t *testing.T, id string) {
		got, err := s.path(id)
		if err != nil {
			if !errors.Is(err, errInvalidID) {
				t.Fatalf("path(%q) error = %v, want errInvalidID", id, err)
			}
			return
		}

		if want := filepath.Join(dir, id, "state.sqlite"); got != want {
			t.Fatalf("path(%q) = %q, want %q", id, got, want)
		}

		rel, relErr := filepath.Rel(dir, got)
		if relErr != nil {
			t.Fatalf("path(%q) = %q: Rel failed: %v", id, got, relErr)
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 2 {
			t.Fatalf("path(%q) rel = %q, want exactly 2 elements", id, rel)
		}
		for _, p := range parts {
			if p == ".." || p == "." || p == "" {
				t.Fatalf("path(%q) rel = %q has element %q", id, rel, p)
			}
		}
		if parts[0] != id || parts[1] != "state.sqlite" {
			t.Fatalf("path(%q) rel = %q, want %q/state.sqlite", id, rel, id)
		}

		// The byte allow-list below excludes '.', '/', '\' and NUL, so an
		// accepted ID can't contain a separator, "..", or ".".
		if len(id) < 1 || len(id) > 40 {
			t.Fatalf("accepted ID %q has length %d, want 1-40", id, len(id))
		}
		for i := 0; i < len(id); i++ {
			c := id[i]
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				t.Fatalf("accepted ID %q has disallowed byte %q at %d", id, c, i)
			}
		}
		if id[0] == '-' {
			t.Fatalf("accepted ID %q starts with '-'", id)
		}
	})
}
