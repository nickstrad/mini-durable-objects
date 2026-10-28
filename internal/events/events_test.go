package events

import (
	"encoding/csv"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var discard = slog.New(slog.DiscardHandler)

func readCSV(t *testing.T, dir, prefix, run string) [][]string {
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
	return rows
}

func join(row []string) string { return strings.Join(row, ",") }

func TestRowsAndHeaders(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir, time.Hour, discard)
	if err != nil {
		t.Fatal(err)
	}
	// Time points are written as offsets from the start of the run.
	start := r.startedAt.Add(5)
	r.Request(Request{Start: start, Object: "a", Op: "query", Kind: "ok", Ran: true, Depth: 2,
		Lock: 1, Open: 2, Queue: 3, ExecAt: r.startedAt.Add(9), Exec: 4, Total: 10, Rows: 7})
	r.Request(Request{Start: start, Object: "b", Op: "query", Kind: "not_found"}) // did not run
	r.Object(Object{At: r.startedAt.Add(200), Object: "a", Event: "evicted", Idle: 5, Lifetime: 6, Requests: 3})
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	run := r.RunID()

	req := readCSV(t, dir, "requests", run)
	if got, want := join(req[0]), "run_id,seq,start_at,object,op,kind,ran,depth,lock_ns,open_ns,queue_ns,exec_at,exec_ns,total_ns,rows"; got != want {
		t.Errorf("requests header = %s", got)
	}
	if got, want := join(req[1]), run+",1,5,a,query,ok,true,2,1,2,3,9,4,10,7"; got != want {
		t.Errorf("requests row = %s, want %s", got, want)
	}
	// A request that did not run has an empty exec_at.
	if got, want := join(req[2]), run+",2,5,b,query,not_found,false,0,0,0,0,,0,0,0"; got != want {
		t.Errorf("not-run row = %s, want %s", got, want)
	}
	obj := readCSV(t, dir, "objects", run)
	if got, want := join(obj[0]), "run_id,seq,object,at,event,open_ns,idle_ns,lifetime_ns,requests"; got != want {
		t.Errorf("objects header = %s", got)
	}
	if got, want := join(obj[1]), run+",3,a,200,evicted,0,5,6,3"; got != want {
		t.Errorf("objects row = %s, want %s", got, want)
	}
	runs := readCSV(t, dir, "runs", run)
	if got, want := join(runs[0]), "run_id,started_at,stopped_at,written,dropped,max_buffered"; got != want {
		t.Errorf("runs header = %s", got)
	}
	if len(runs) != 2 || runs[1][3] != "3" || runs[1][4] != "0" {
		t.Errorf("runs = %v, want written 3 dropped 0", runs)
	}
}

func TestFlushOnTicker(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir, 10*time.Millisecond, discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.Request(Request{Object: "a", Op: "create", Kind: "ok"})
	deadline := time.Now().Add(5 * time.Second)
	for {
		// The file has rows before Close only if the ticker flushed.
		b, _ := os.ReadFile(filepath.Join(dir, "requests-"+r.RunID()+".csv"))
		if strings.Count(string(b), "\n") == 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("event was not flushed by the ticker")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDropWhenFull(t *testing.T) {
	dir := t.TempDir()
	r, err := open(dir, time.Hour, discard, 4) // no writer yet, so the buffer fills
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		r.Request(Request{Object: "a", Op: "create", Kind: "ok"})
	}
	go r.write()
	for len(r.ch) > 0 {
		time.Sleep(time.Millisecond)
	}
	r.Object(Object{Object: "a", Event: "activated"}) // seq 11, after the gap
	r.Close()

	run := r.RunID()
	var seqs []string
	for _, row := range readCSV(t, dir, "requests", run)[1:] {
		seqs = append(seqs, row[1])
	}
	for _, row := range readCSV(t, dir, "objects", run)[1:] {
		seqs = append(seqs, row[1])
	}
	if got := strings.Join(seqs, ","); got != "1,2,3,4,11" {
		t.Errorf("seqs = %s, want 1,2,3,4,11", got)
	}
	runs := readCSV(t, dir, "runs", run)[1]
	if runs[3] != "5" || runs[4] != "6" || runs[5] != "4" {
		t.Errorf("written, dropped, max_buffered = %v, want 5, 6, 4", runs[3:])
	}
}

func TestEventAfterCloseIsDropped(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir, time.Hour, discard)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	r.Request(Request{Object: "a"}) // must not panic
	if got := r.dropped.Load(); got != 1 {
		t.Errorf("dropped = %d, want 1", got)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close = %v", err)
	}
}

func TestNilRecorder(t *testing.T) {
	var r *Recorder
	r.Request(Request{})
	r.Object(Object{})
	if err := r.Close(); err != nil {
		t.Error(err)
	}
	if r.RunID() != "" {
		t.Error("nil recorder has a run ID")
	}
}
