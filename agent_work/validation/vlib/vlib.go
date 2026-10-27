// Package vlib holds the shared plumbing for the validation scripts: copying
// the repo to a temp directory, applying string replacements to the copy, and
// running go test there. Nothing here touches the source repo.
package vlib

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// skip lists top-level entries that are not copied: VCS metadata, local
// databases, built binaries, and this validation folder itself. Keep in sync
// with the rsync excludes in flaky.sh.
var skip = map[string]bool{".git": true, "data": true, "bin": true, "agent_work": true}

// CopyRepo copies src into a fresh temp directory and returns its path and a
// cleanup func. Symlinks are not followed; the repo has none.
func CopyRepo(src string) (dst string, cleanup func(), err error) {
	src, err = filepath.Abs(src)
	if err != nil {
		return "", nil, err
	}
	if _, err := os.Stat(filepath.Join(src, "go.mod")); err != nil {
		return "", nil, fmt.Errorf("%s does not look like the repo root: %w", src, err)
	}
	dst, err = os.MkdirTemp("", "mdo-validate-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(dst) }
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		if d.IsDir() && skip[rel] {
			return filepath.SkipDir
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, info.Mode().Perm())
	})
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return dst, cleanup, nil
}

// Replace applies the {old, new} pairs in order to the file at path and
// returns the original contents so the caller can restore them. Each old must
// occur exactly once; otherwise Replace returns an error and writes nothing.
func Replace(path string, pairs [][2]string) (orig []byte, err error) {
	orig, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := string(orig)
	for _, p := range pairs {
		if n := strings.Count(s, p[0]); n != 1 {
			return orig, fmt.Errorf("%s: pattern %q occurs %d times, want 1", filepath.Base(path), p[0], n)
		}
		s = strings.Replace(s, p[0], p[1], 1)
	}
	return orig, os.WriteFile(path, []byte(s), 0o644)
}

// GoTest runs `go test` in dir with args and returns combined output and
// whether it passed. A run longer than timeout is killed and counts as failed.
func GoTest(dir string, timeout time.Duration, args ...string) (out string, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"test"}, args...)...)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	out = string(b)
	if ctx.Err() != nil {
		out += "\n[validation: killed after " + timeout.String() + "]"
	}
	return out, err == nil
}

// Summarize pulls the interesting lines out of go test output: failing test
// names, assertion messages, panics, and timeouts.
func Summarize(out string, max int) []string {
	var lines []string
	seen := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "--- FAIL"),
			strings.HasPrefix(t, "panic:"),
			strings.Contains(t, "_test.go:"),
			strings.Contains(t, "[validation:"),
			strings.Contains(t, "build failed"):
		default:
			continue
		}
		if len(t) > 160 {
			t = t[:157] + "..."
		}
		if !seen[t] {
			seen[t] = true
			lines = append(lines, t)
		}
		if len(lines) == max {
			break
		}
	}
	return lines
}
