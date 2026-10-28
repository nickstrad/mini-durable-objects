// Package events records opt-in runtime events as CSV files, for analysis
// with a tool such as DuckDB. CODING_STYLE.md ("Events") gives the schema.
//
// Emitting an event never waits and never does I/O: it sends a small struct to
// a buffered channel. One goroutine formats the rows and writes the files. If
// the channel is full, the event is dropped and counted. A nil *Recorder does
// nothing.
package events

import (
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const bufferSize = 4096

var (
	requestsHeader = []string{"run_id", "seq", "start_at", "object", "op", "kind", "ran", "depth",
		"lock_ns", "open_ns", "queue_ns", "exec_at", "exec_ns", "total_ns", "rows"}
	objectsHeader = []string{"run_id", "seq", "object", "at", "event", "open_ns", "idle_ns", "lifetime_ns", "requests"}
	runsHeader    = []string{"run_id", "started_at", "stopped_at", "written", "dropped", "max_buffered"}
)

// Request is one call to Store.Create or Store.Query.
type Request struct {
	Start  time.Time
	Object string
	Op     string // "create" or "query"
	Kind   string // "ok" or an error Kind
	Ran    bool
	Depth  int
	Lock   time.Duration
	Open   time.Duration
	Queue  time.Duration
	ExecAt time.Time // when the object goroutine started the request; zero if it did not run
	Exec   time.Duration
	Total  time.Duration
	Rows   int
}

// Object is one activation or eviction of an object.
type Object struct {
	At       time.Time
	Object   string
	Event    string // "activated" or "evicted"
	Open     time.Duration
	Idle     time.Duration
	Lifetime time.Duration
	Requests int
}

// entry is what travels through the channel: one event with its sequence number.
type entry struct {
	seq   int64
	isObj bool
	req   Request
	obj   Object
}

// Recorder writes events to CSV files in one directory.
type Recorder struct {
	runID     string
	startedAt time.Time // time points in the files are offsets from this
	dir       string
	flush     time.Duration
	logger    *slog.Logger

	seq     atomic.Int64
	dropped atomic.Int64

	// mu makes Close wait for senders, so nobody sends on a closed channel.
	mu        sync.RWMutex
	closed    bool
	ch        chan entry
	done      chan struct{} // closed when the writer goroutine ends
	closeOnce sync.Once     // a second Close, even a concurrent one, waits for the first
	closeErr  error

	// Owned by the writer goroutine until done is closed.
	reqs, objs  *csv.Writer
	files       []*os.File
	written     int64
	maxBuffered int64
}

// Open makes dir, picks a run ID, writes the two file headers, and starts the
// writer goroutine. It flushes the files every flush.
func Open(dir string, flush time.Duration, logger *slog.Logger) (*Recorder, error) {
	r, err := open(dir, flush, logger, bufferSize)
	if err != nil {
		return nil, err
	}
	go r.write()
	return r, nil
}

// open makes a Recorder without starting its writer goroutine.
func open(dir string, flush time.Duration, logger *slog.Logger, size int) (*Recorder, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create events dir: %w", err)
	}
	now := time.Now()
	r := &Recorder{
		runID:     now.UTC().Format("20060102T150405Z") + "-" + rand.Text()[:6],
		startedAt: now,
		dir:       dir,
		flush:     flush,
		logger:    logger,
		ch:        make(chan entry, size),
		done:      make(chan struct{}),
	}
	var err error
	if r.reqs, err = r.create("requests", requestsHeader); err == nil {
		r.objs, err = r.create("objects", objectsHeader)
	}
	if err != nil {
		r.closeFiles()
		return nil, err
	}
	return r, nil
}

// create makes one events file for this run and writes its header.
func (r *Recorder) create(name string, header []string) (*csv.Writer, error) {
	f, err := os.Create(filepath.Join(r.dir, name+"-"+r.runID+".csv"))
	if err != nil {
		return nil, fmt.Errorf("create events file: %w", err)
	}
	r.files = append(r.files, f)
	w := csv.NewWriter(f)
	w.Write(header)
	w.Flush()
	return w, w.Error()
}

// RunID returns the run ID that is in the file names.
func (r *Recorder) RunID() string {
	if r == nil {
		return ""
	}
	return r.runID
}

// Request records a request event. It never waits.
func (r *Recorder) Request(e Request) {
	if r != nil {
		r.send(entry{req: e})
	}
}

// Object records an object event. It never waits.
func (r *Recorder) Object(e Object) {
	if r != nil {
		r.send(entry{isObj: true, obj: e})
	}
}

func (r *Recorder) send(e entry) {
	e.seq = r.seq.Add(1)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		r.dropped.Add(1)
		return
	}
	select {
	case r.ch <- e:
	default:
		r.dropped.Add(1)
	}
}

// write is the writer goroutine. It formats and writes rows, and flushes on a
// ticker, until Close closes the channel.
func (r *Recorder) write() {
	defer close(r.done)
	tick := time.NewTicker(r.flush)
	defer tick.Stop()
	for {
		select {
		case e, ok := <-r.ch:
			if !ok {
				return
			}
			// This event and the ones still in the channel were buffered together.
			r.maxBuffered = max(r.maxBuffered, int64(len(r.ch)+1))
			r.writeRow(e)
		case <-tick.C:
			r.flushFiles()
		}
	}
}

func (r *Recorder) writeRow(e entry) {
	run, seq := r.runID, strconv.FormatInt(e.seq, 10)
	if e.isObj {
		o := e.obj
		r.objs.Write([]string{run, seq, o.Object, r.at(o.At), o.Event,
			ns(o.Open), ns(o.Idle), ns(o.Lifetime), strconv.Itoa(o.Requests)})
	} else {
		q := e.req
		r.reqs.Write([]string{run, seq, r.at(q.Start), q.Object, q.Op, q.Kind,
			strconv.FormatBool(q.Ran), strconv.Itoa(q.Depth),
			ns(q.Lock), ns(q.Open), ns(q.Queue), r.at(q.ExecAt), ns(q.Exec), ns(q.Total), strconv.Itoa(q.Rows)})
	}
	r.written++
}

// at writes a time point as nanoseconds since the run started. Both times come
// from the monotonic clock, so a point and a duration can be added exactly.
// The zero time is an empty field (NULL in DuckDB).
func (r *Recorder) at(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return ns(t.Sub(r.startedAt))
}

func ns(d time.Duration) string { return strconv.FormatInt(d.Nanoseconds(), 10) }

func (r *Recorder) flushFiles() {
	for _, w := range []*csv.Writer{r.reqs, r.objs} {
		w.Flush()
		if err := w.Error(); err != nil {
			r.logger.Error("events write failed", "err", err)
		}
	}
}

func (r *Recorder) closeFiles() {
	for _, f := range r.files {
		f.Close()
	}
}

// Close stops intake, writes the events in the buffer, and writes the runs
// row. Events sent after Close are dropped and counted. Later calls do nothing.
func (r *Recorder) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		close(r.ch)
		r.mu.Unlock()
		<-r.done
		r.flushFiles()
		r.closeErr = r.writeRun()
		r.closeFiles()
	})
	return r.closeErr
}

// writeRun writes the runs file. Its times are Unix nanoseconds: they anchor the
// offsets in the other files to the wall clock.
func (r *Recorder) writeRun() error {
	f, err := os.Create(filepath.Join(r.dir, "runs-"+r.runID+".csv"))
	if err != nil {
		return fmt.Errorf("create runs file: %w", err)
	}
	stopped := r.startedAt.Add(time.Since(r.startedAt))
	w := csv.NewWriter(f)
	w.Write(runsHeader)
	w.Write([]string{r.runID, strconv.FormatInt(r.startedAt.UnixNano(), 10), strconv.FormatInt(stopped.UnixNano(), 10),
		strconv.FormatInt(r.written, 10), strconv.FormatInt(r.dropped.Load(), 10), strconv.FormatInt(r.maxBuffered, 10)})
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
