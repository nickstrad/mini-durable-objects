package store

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
)

func TestRegistryReusesWarmObject(t *testing.T) {
	r := newFakeRegistry(t, time.Hour)
	path := "a"

	var first, second *fakeRes
	if err := r.do(t.Context(), "a", path, func(db *fakeRes) error { first = db; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.do(t.Context(), "a", path, func(db *fakeRes) error { second = db; return nil }); err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("warm object was reopened")
	}
}

func TestRegistryEvictsOnlyIdleObjects(t *testing.T) {
	r := newFakeRegistry(t, 0) // Every released object is immediately idle.

	a, err := r.acquire("a", "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.acquire("b", "b")
	if err != nil {
		t.Fatal(err)
	}
	r.release(b)
	if _, ok := r.objects["a"]; !ok {
		t.Fatal("object in use was evicted")
	}

	r.release(a)
	b, err = r.acquire("b", "b")
	if err != nil {
		t.Fatal(err)
	}

	r.release(b)
	if _, ok := r.objects["a"]; ok {
		t.Fatal("idle object was not evicted")
	}
}

func TestRegistryRunsOneRequestAtATimePerObject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, 0)
		ctx := context.Background()

		ran := make(chan string, 3)
		unblock := make(chan struct{})
		go r.do(ctx, "a", "a", func(*fakeRes) error {
			ran <- "a first"
			<-unblock
			return nil
		})
		synctest.Wait() // a's first request is running and blocked.

		// Another object does not wait on a.
		r.do(ctx, "b", "b", func(*fakeRes) error { ran <- "b"; return nil })

		go r.do(ctx, "a", "a", func(*fakeRes) error { ran <- "a second"; return nil })
		synctest.Wait() // a's second request is waiting in the inbox.
		if got := drain(ran); !slices.Equal(got, []string{"a first", "b"}) {
			t.Fatalf("before release ran %v", got)
		}

		close(unblock)
		synctest.Wait()
		if got := drain(ran); !slices.Equal(got, []string{"a second"}) {
			t.Fatalf("after release ran %v", got)
		}

	})
}

func TestRegistryCanceledRequestLeavesInbox(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, 0)
		path := "a"

		unblock := make(chan struct{})
		go r.do(context.Background(), "a", path, func(*fakeRes) error { <-unblock; return nil })
		synctest.Wait() // a is busy.

		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		ran := false
		go func() {
			errc <- r.do(ctx, "a", path, func(*fakeRes) error { ran = true; return nil })
		}()
		synctest.Wait() // The second request is waiting in the inbox.

		cancel()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
		close(unblock)
		synctest.Wait()
		if ran {
			t.Fatal("canceled request still ran")
		}

	})
}

// drain returns everything currently in ch without waiting.
func drain(ch chan string) []string {
	var got []string
	for {
		select {
		case s := <-ch:
			got = append(got, s)
		default:
			return got
		}
	}
}

// fakeRes stands in for an object's database. The registry only opens,
// hands out, and closes its resource, so a fake lets these tests check
// closing without SQLite. Paths are only labels here.
type fakeRes struct {
	path   string
	closes atomic.Int32
}

func (f *fakeRes) Close() error {
	f.closes.Add(1)
	return nil
}

// discard drops all log records.
var discard = slog.New(slog.DiscardHandler)

// newFakeRegistry returns a registry of fakes that is stopped when the test
// ends.
func newFakeRegistry(t *testing.T, idleTimeout time.Duration) *registry[*fakeRes] {
	r := newRegistry(idleTimeout, func(path string) (*fakeRes, error) {
		return &fakeRes{path: path}, nil
	}, discard)
	t.Cleanup(func() { stopAll(r) })
	return r
}

// stopAll evicts every idle object so its goroutine exits, which synctest
// requires before the test can end. Inside synctest.Test, cleanups run in
// the bubble before it checks for leftover goroutines.
func stopAll(r *registry[*fakeRes]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.idleTimeout = 0
	r.evictIdle()
}

func TestRegistryWaitingRequestKeepsObjectAlive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, 0) // Every released object is immediately idle.
		ctx := context.Background()

		unblock := make(chan struct{})
		go r.do(ctx, "a", "a", func(*fakeRes) error { <-unblock; return nil })
		synctest.Wait() // a's first request is running and blocked.

		secondRan := false
		go r.do(ctx, "a", "a", func(*fakeRes) error { secondRan = true; return nil })
		synctest.Wait() // a's second request is waiting in the inbox.

		// A request to b runs eviction. a has a request in line, so it stays.
		if err := r.do(ctx, "b", "b", func(*fakeRes) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if _, ok := r.objects["a"]; !ok {
			t.Fatal("object with a waiting request was evicted")
		}

		close(unblock)
		synctest.Wait()
		if !secondRan {
			t.Fatal("waiting request did not run")
		}

	})
}

func TestRegistryReturnsFnError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, time.Hour)
		path := "a"

		want := errors.New("boom")
		err := r.do(context.Background(), "a", path, func(*fakeRes) error { return want })
		if !errors.Is(err, want) {
			t.Fatalf("got %v, want %v", err, want)
		}

	})
}

func TestRegistryAlreadyCanceledRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, time.Hour)
		path := "a"

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ran := false
		err := r.do(ctx, "a", path, func(*fakeRes) error { ran = true; return nil })

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
		if ran {
			t.Fatal("already-canceled request ran")
		}
		if _, ok := r.objects["a"]; ok {
			t.Fatal("already-canceled request activated the object")
		}

	})
}

func TestRegistryReactivatesWithFreshDB(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, 0) // Every released object is immediately idle.
		ctx := context.Background()

		var first, second *fakeRes
		if err := r.do(ctx, "a", "a", func(db *fakeRes) error { first = db; return nil }); err != nil {
			t.Fatal(err)
		}
		// A request to b evicts idle a.
		if err := r.do(ctx, "b", "b", func(*fakeRes) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if _, ok := r.objects["a"]; ok {
			t.Fatal("idle object was not evicted")
		}
		if err := r.do(ctx, "a", "a", func(db *fakeRes) error { second = db; return nil }); err != nil {
			t.Fatal(err)
		}
		if first == second {
			t.Fatal("reactivated object reused its closed database")
		}
		synctest.Wait() // Let the evicted goroutine finish closing.
		if n := first.closes.Load(); n != 1 {
			t.Fatalf("evicted resource closed %d times, want 1", n)
		}
		if n := second.closes.Load(); n != 0 {
			t.Fatalf("active resource closed %d times, want 0", n)
		}

	})
}

// TestRegistryRandomWorkloads runs random mixes of requests, some canceled,
// against several objects and checks the registry's promises:
//
//   - per object, at most one fn runs at a time;
//   - do returns nil exactly when fn ran, once; do returns ctx.Err() exactly
//     when fn never ran;
//   - a blocked object does not stall requests to other objects;
//   - a request whose ctx is already done never waits in line for a busy object;
//   - an object idle for less than idleTimeout stays active;
//   - afterwards no request is counted as active, and once everything is
//     evicted every object goroutine exits (synctest.Test fails otherwise);
//   - a resource is never closed while a request uses it, and every resource
//     ever opened is closed exactly once in the end.
//
// Order of waiting requests is not checked; the registry does not promise one.
//
// Time is synctest's fake clock, so the sleeps below cost no real time. A
// failing seed is logged; scheduling still varies between runs, so a seed
// makes a failure likely to repeat rather than certain.
func TestRegistryRandomWorkloads(t *testing.T) {
	for seed := range uint64(200) {
		synctest.Test(t, func(t *testing.T) {
			regRandomWorkload(t, seed)
		})
		if t.Failed() {
			t.Fatalf("failed with seed %d", seed)
		}
	}
}

func regRandomWorkload(t *testing.T, seed uint64) {
	rng := rand.New(rand.NewPCG(seed, seed))

	// Idle timeout 0 makes objects churn through evict and reactivate.
	idle := []time.Duration{0, time.Millisecond, time.Hour}[rng.IntN(3)]

	// Remember every resource opened, to check each is closed exactly once.
	var openedMu sync.Mutex
	var opened []*fakeRes
	r := newRegistry(idle, func(path string) (*fakeRes, error) {
		res := &fakeRes{path: path}
		openedMu.Lock()
		opened = append(opened, res)
		openedMu.Unlock()
		return res, nil
	}, discard)

	ids := []string{"a", "b", "c", "d"}[:2+rng.IntN(3)]
	inFlight := make(map[string]*atomic.Int32)
	for _, id := range ids {
		inFlight[id] = new(atomic.Int32)
	}
	blockedID := ids[0] // Held busy until every other object's requests finish.

	// Block blockedID before the workload starts.
	unblock := make(chan struct{})
	blockedDone := make(chan error, 1)
	go func() {
		blockedDone <- r.do(context.Background(), blockedID, blockedID, func(*fakeRes) error {
			inFlight[blockedID].Add(1)
			defer inFlight[blockedID].Add(-1)
			<-unblock
			return nil
		})
	}()
	synctest.Wait()

	type request struct {
		id          string
		preCanceled bool
		runs        atomic.Int32
		err         error
	}
	reqs := make([]*request, 10+rng.IntN(50))
	var others, blocked, canceledWhileBlocked sync.WaitGroup
	for i := range reqs {
		req := &request{id: ids[rng.IntN(len(ids))]}
		reqs[i] = req

		ctx, cancel := context.WithCancel(context.Background())
		mode := rng.IntN(3)
		switch mode {
		case 0: // Never canceled.
		case 1: // Canceled before do is called.
			cancel()
			req.preCanceled = true
		case 2: // Canceled at a random later time.
			d := time.Duration(rng.IntN(20)) * time.Millisecond
			go func() { time.Sleep(d); cancel() }()
		}
		start := time.Duration(rng.IntN(20)) * time.Millisecond
		work := time.Duration(rng.IntN(5)) * time.Millisecond

		wg := &others
		switch {
		case req.id == blockedID && mode == 1:
			// The object is busy and ctx is already done, so do must return
			// without waiting in line.
			wg = &canceledWhileBlocked
		case req.id == blockedID:
			wg = &blocked
		}
		wg.Go(func() {
			defer cancel()
			time.Sleep(start)
			req.err = r.do(ctx, req.id, req.id, func(res *fakeRes) error {
				req.runs.Add(1)
				if res.closes.Load() != 0 {
					t.Errorf("seed %d: request %d on %s got a closed resource", seed, i, req.id)
				}
				if n := inFlight[req.id].Add(1); n > 1 {
					t.Errorf("seed %d: %d fns running at once on %s", seed, n, req.id)
				}
				time.Sleep(work)
				inFlight[req.id].Add(-1)
				return nil
			})
		})
	}

	// If blockedID stalled other objects, this would deadlock and
	// synctest.Test would fail the test.
	others.Wait()
	// Deadlocks, failing the test, if a canceled request waited on the busy inbox.
	canceledWhileBlocked.Wait()
	close(unblock)
	blocked.Wait()
	if err := <-blockedDone; err != nil {
		t.Errorf("seed %d: blocking request: %v", seed, err)
	}

	for i, req := range reqs {
		runs := req.runs.Load()
		switch {
		case req.preCanceled && (runs != 0 || !errors.Is(req.err, context.Canceled)):
			t.Errorf("seed %d: request %d was canceled before do but ran %d times, err %v", seed, i, runs, req.err)
		case req.err == nil && runs != 1:
			t.Errorf("seed %d: request %d returned nil but ran %d times", seed, i, runs)
		case errors.Is(req.err, context.Canceled) && runs != 0:
			t.Errorf("seed %d: request %d was canceled but ran %d times", seed, i, runs)
		case req.err != nil && !errors.Is(req.err, context.Canceled):
			t.Errorf("seed %d: request %d: unexpected error %v", seed, i, req.err)
		}
	}

	r.mu.Lock()
	if idle == time.Hour {
		for _, req := range reqs {
			if _, ok := r.objects[req.id]; !ok && req.err == nil {
				t.Errorf("seed %d: %s was evicted although idle for less than %v", seed, req.id, idle)
			}
		}
	}
	for id, obj := range r.objects {
		if obj.active != 0 {
			t.Errorf("seed %d: %s has active = %d after all requests returned", seed, id, obj.active)
		}
	}
	r.mu.Unlock()

	stopAll(r)
	synctest.Wait() // Evicted goroutines close their resources.
	openedMu.Lock()
	defer openedMu.Unlock()
	for _, res := range opened {
		if n := res.closes.Load(); n != 1 {
			t.Errorf("seed %d: resource %s closed %d times, want 1", seed, res.path, n)
		}
	}
}

func TestRegistryReturnsOpenError(t *testing.T) {
	want := errors.New("open failed")
	r := newRegistry(time.Hour, func(string) (*fakeRes, error) { return nil, want }, discard)

	err := r.do(t.Context(), "a", "a", func(*fakeRes) error {
		t.Error("fn ran without a resource")
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if _, ok := r.objects["a"]; ok {
		t.Fatal("failed activation left an object registered")
	}
}

func TestRegistryRecoversPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newFakeRegistry(t, time.Hour)

		err := r.do(context.Background(), "a", "a", func(*fakeRes) error { panic("boom") })
		e := wrapped(t, err)
		if e.Kind != errs.Internal {
			t.Fatalf("got %v, want Internal", err)
		}
		if !bytes.Contains(e.Stack, []byte("panic")) {
			t.Fatalf("stack does not show the panic:\n%s", e.Stack)
		}
		// The object still serves requests.
		ran := false
		if err := r.do(context.Background(), "a", "a", func(*fakeRes) error { ran = true; return nil }); err != nil || !ran {
			t.Fatalf("after panic: ran %v, err %v", ran, err)
		}
	})
}

func TestRegistryLogsActivateAndEvict(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	r := newRegistry(0, func(path string) (*fakeRes, error) { return &fakeRes{path: path}, nil }, logger)
	t.Cleanup(func() { stopAll(r) })

	do := func(id string) {
		if err := r.do(t.Context(), id, id, func(*fakeRes) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	do("a")
	do("b") // The idle timeout is 0, so this evicts a.
	out := buf.String()
	for _, want := range []string{"msg=\"object activated\" object=a", "msg=\"object evicted\" object=a", "msg=\"object activated\" object=b"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q: %s", want, out)
		}
	}
}
