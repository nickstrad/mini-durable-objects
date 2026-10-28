package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
	"github.com/nickstrad/mini-durable-objects/internal/events"
)

// registry keeps each object's resource (its database, for SQLite) open
// between requests. The first request activates the object (cold); later
// requests reuse it (warm). An object nobody has used for idleTimeout is
// closed and forgotten. The registry knows nothing about storage: it only
// opens, hands out, and closes R.
//
// Each active object is one goroutine that owns its resource. Requests are
// sent to its inbox and run one at a time, while different objects run in
// parallel. Waiting requests are usually served in arrival order, but Go does
// not promise which blocked sender a channel serves next, so neither does the
// registry. mu guards only the map and counts.
type registry[R io.Closer] struct {
	idleTimeout time.Duration
	open        func(path string) (R, error)
	logger      *slog.Logger
	rec         *events.Recorder // nil turns events off

	mu      sync.Mutex
	objects map[string]*object[R]
}

type object[R io.Closer] struct {
	inbox    chan<- func(R)  // requests for the object's goroutine
	done     chan<- struct{} // closed by eviction to stop the goroutine
	active   int             // requests running or waiting in inbox
	lastUsed time.Time

	activatedAt time.Time
	requests    int // requests run since activation
}

// timing is what do measures for one request. The store turns it into an event.
type timing struct {
	Ran                     bool      // the object goroutine ran fn
	Depth                   int       // requests active for the object when this one arrived
	ExecAt                  time.Time // when the object goroutine started fn
	Lock, Open, Queue, Exec time.Duration
}

// newRegistry makes a registry that activates objects with open.
func newRegistry[R io.Closer](idleTimeout time.Duration, open func(path string) (R, error), logger *slog.Logger, rec *events.Recorder) *registry[R] {
	return &registry[R]{idleTimeout: idleTimeout, open: open, logger: logger, rec: rec, objects: make(map[string]*object[R])}
}

// do runs fn with id's resource on the object's goroutine, opening path if the
// object is not active. No other request for the object runs at the same time.
//
// A request already canceled when do is called never runs. One canceled while
// waiting in line leaves without running. If cancellation lands just as the
// object accepts fn, fn may still run; do returns nil exactly when fn ran.
func (r *registry[R]) do(ctx context.Context, id, path string, fn func(R) error) error {
	return r.doTimed(ctx, id, path, fn, new(timing))
}

// doTimed is do that also fills t with what it measured. The object goroutine
// writes some fields; doTimed reads them only after it gets the result.
func (r *registry[R]) doTimed(ctx context.Context, id, path string, fn func(R) error, t *timing) error {
	// A dead request should not activate the object or run fn. (The select
	// below can't promise that alone: when the inbox is also ready, it picks
	// a ready case at random.)
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := r.acquire(id, path, t)
	if err != nil {
		return err
	}
	defer r.release(obj, t)

	// Buffered so the object's goroutine never waits on the reply.
	result := make(chan error, 1)
	sent := time.Now()
	select {
	case obj.inbox <- func(res R) { result <- timedCall(t, sent, fn, res) }:
	case <-ctx.Done():
		// Canceled while waiting in line; the object never received fn.
		return ctx.Err()
	}
	return <-result
}

// timedCall runs fn on the object's goroutine and records the queue and exec
// times in tm. do reads tm only after it gets the result, so there is no race.
func timedCall[R any](tm *timing, sent time.Time, fn func(R) error, res R) error {
	started := time.Now()
	tm.Ran, tm.ExecAt, tm.Queue = true, started, started.Sub(sent)
	err := call(fn, res)
	tm.Exec = time.Since(started)
	return err
}

// call runs fn. A panic in fn becomes an Internal error with the panic's stack,
// so one bad request does not stop the process or the object's goroutine.
// The wrap happens here, inside the recover, only so the stack shows the
// panicking frames; wrapErr sets the Op and Message the client sees.
func call[R any](fn func(R) error, res R) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errs.Wrap(fmt.Errorf("panic: %v", p), errs.Internal, "store.registry", "storage error")
		}
	}()
	return fn(res)
}

// acquire returns id's object, activating it if needed. Call release when done
// so the object can be evicted once idle.
func (r *registry[R]) acquire(id, path string, tm *timing) (*object[R], error) {
	lockStart := time.Now()
	r.mu.Lock()
	tm.Lock = time.Since(lockStart)
	defer r.mu.Unlock()
	r.evictIdle()

	obj, ok := r.objects[id]
	if !ok {
		openStart := time.Now()
		res, err := r.open(path)
		if err != nil {
			return nil, err
		}
		tm.Open = time.Since(openStart)
		inbox := make(chan func(R))
		done := make(chan struct{})
		go run(done, inbox, res)
		obj = &object[R]{inbox: inbox, done: done, activatedAt: time.Now()}
		r.objects[id] = obj
		r.logger.Info("object activated", "object", id)
		r.rec.Object(events.Object{At: obj.activatedAt, Object: id, Event: "activated", Open: tm.Open})
	}
	// Count the request before it waits in the inbox, so the object is not
	// evicted while the request is in line.
	tm.Depth = obj.active
	obj.active++
	return obj, nil
}

// release ends a request that acquire started. t.Ran says if fn ran, for
// the requests count of the activation.
func (r *registry[R]) release(obj *object[R], t *timing) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.Ran {
		obj.requests++
	}
	obj.active--
	obj.lastUsed = time.Now()
}

// run is an object's goroutine. Only it uses res, so the object's requests run
// one at a time. It exits, closing res, when done is closed.
func run[R io.Closer](done <-chan struct{}, inbox <-chan func(R), res R) {
	defer res.Close()
	for {
		select {
		case <-done:
			return
		case fn := <-inbox:
			fn(res)
		}
	}
}

// evictIdle stops objects that are not in use and have been idle too long.
// It is lazy: eviction only happens when a request comes in.
//
// The inbox is never closed, so a late send cannot panic. An object is only
// stopped when active is 0, which means no request is sending or waiting.
func (r *registry[R]) evictIdle() {
	now := time.Now()
	for id, obj := range r.objects {
		if obj.active == 0 && now.Sub(obj.lastUsed) >= r.idleTimeout {
			close(obj.done)
			delete(r.objects, id)
			r.logger.Info("object evicted", "object", id)
			r.rec.Object(events.Object{At: now, Object: id, Event: "evicted", Idle: now.Sub(obj.lastUsed),
				Lifetime: now.Sub(obj.activatedAt), Requests: obj.requests})
		}
	}
}
