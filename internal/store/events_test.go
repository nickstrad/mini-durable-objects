package store

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/events"
)

func readEvents(t *testing.T, dir, prefix, run string) []map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, prefix+"-"+run+".csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]string
	for _, row := range rows[1:] {
		m := map[string]string{}
		for i, h := range rows[0] {
			m[h] = row[i]
		}
		out = append(out, m)
	}
	return out
}

func TestStoreEmitsEvents(t *testing.T) {
	evDir := t.TempDir()
	rec, err := events.Open(evDir, time.Hour, discard)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSQLite(t.TempDir(), 20*time.Millisecond, discard, rec)
	ctx := context.Background()
	s.Create(ctx, "BAD ID")
	s.Create(ctx, "a")
	s.Query(ctx, "a", "select 1 union select 2", nil)
	s.Query(ctx, "a", "nonsense", nil)
	s.Query(ctx, "missing", "select 1", nil)
	time.Sleep(40 * time.Millisecond)
	s.Create(ctx, "b") // evicts a
	rec.Close()

	reqs := readEvents(t, evDir, "requests", rec.RunID())
	want := []struct{ op, kind, ran, rows string }{
		{"create", "invalid", "false", "0"},
		{"create", "ok", "true", "0"},
		{"query", "ok", "true", "2"},
		{"query", "bad_query", "true", "0"},
		{"query", "not_found", "false", "0"},
		{"create", "ok", "true", "0"},
	}
	if len(reqs) != len(want) {
		t.Fatalf("got %d request events, want %d", len(reqs), len(want))
	}
	for i, w := range want {
		g := reqs[i]
		if g["op"] != w.op || g["kind"] != w.kind || g["ran"] != w.ran || g["rows"] != w.rows {
			t.Errorf("request %d = %v, want %+v", i, g, w)
		}
		sum := 0
		for _, c := range []string{"lock_ns", "open_ns", "queue_ns", "exec_ns"} {
			n, _ := strconv.Atoi(g[c])
			sum += n
		}
		total, _ := strconv.Atoi(g["total_ns"])
		if sum > total {
			t.Errorf("request %d: steps %d exceed total %d", i, sum, total)
		}
		if w.ran == "false" && sum != 0 {
			t.Errorf("request %d did not run but has timings", i)
		}
	}
	if reqs[1]["open_ns"] == "0" || reqs[2]["open_ns"] != "0" {
		t.Errorf("open_ns: cold = %s, warm = %s", reqs[1]["open_ns"], reqs[2]["open_ns"])
	}

	var got []string
	for _, o := range readEvents(t, evDir, "objects", rec.RunID()) {
		got = append(got, o["object"]+" "+o["event"]+" "+o["requests"])
	}
	// a ran 3 requests (create, ok query, bad query); the missing object never activated.
	want2 := []string{"a activated 0", "a evicted 3", "b activated 0"}
	if len(got) != len(want2) {
		t.Fatalf("object events = %v, want %v", got, want2)
	}
	for i := range want2 {
		if got[i] != want2[i] {
			t.Errorf("object events = %v, want %v", got, want2)
		}
	}
}
