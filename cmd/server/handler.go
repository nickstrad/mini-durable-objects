package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nickstrad/mini-durable-objects/internal/errs"
	"github.com/nickstrad/mini-durable-objects/internal/store"
)

type Object struct {
	ID string `json:"id"`
}

// handler maps HTTP requests to Store calls. It passes /sql requests through
// without inspecting the SQL; the store decides what is allowed.
type handler struct {
	store  store.Store
	logger *slog.Logger
}

func newRouter(s store.Store, logger *slog.Logger) http.Handler {
	h := handler{store: s, logger: logger}
	mux := chi.NewRouter()
	mux.Post("/objects/{id}", h.create)
	mux.Post("/objects/{id}/sql", h.sql)
	return logRequests(logger, mux)
}

// statusWriter remembers the status code that the handler sends.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests writes one Debug record for each request. It never logs the body.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Debug("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration", time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

// writeError is the top of the server. It logs the error once with a log ID and
// shows the client only the Message and that ID.
func (h handler) writeError(w http.ResponseWriter, err error) {
	id := rand.Text()

	e, ok := errors.AsType[*errs.Error](err)
	if !ok {
		h.logger.Error("unwrapped error reached the top", "log_id", id, "err", err.Error())
		http.Error(w, "internal error (log id "+id+")", http.StatusInternalServerError)
		return
	}
	// One switch picks both the status and the log level: a bug is Error, a
	// canceled request is Warn, a client mistake is Info.
	code, level := http.StatusInternalServerError, slog.LevelError
	switch e.Kind {
	case errs.Invalid, errs.BadQuery:
		code, level = http.StatusBadRequest, slog.LevelInfo
	case errs.NotFound:
		code, level = http.StatusNotFound, slog.LevelInfo
	case errs.Canceled:
		code, level = http.StatusServiceUnavailable, slog.LevelWarn
	}

	attrs := []any{"log_id", id, "kind", e.Kind.String(), "op", e.Op, "err", e.Error()}
	// Each Misc entry is its own key, for example object=alpha. Sorted, so the
	// order is the same each time.
	for _, key := range slices.Sorted(maps.Keys(e.Misc)) {
		attrs = append(attrs, key, e.Misc[key])
	}
	// Only Internal errors carry a stack (see errs.Wrap).
	if len(e.Stack) > 0 {
		attrs = append(attrs, "stack", string(e.Stack))
	}
	h.logger.Log(context.Background(), level, "request failed", attrs...)
	http.Error(w, e.Message+" (log id "+id+")", code)
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Create(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, Object{ID: id})
}

func (h handler) sql(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		SQL    string `json:"sql"`
		Params []any  `json:"params"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.UseNumber() // Keep integers exact instead of turning them into float64.
	if err := dec.Decode(&body); err != nil {
		h.writeError(w, errs.Wrap(err, errs.Invalid, "server.sql", "invalid JSON").With("object", id))
		return
	}
	for i, p := range body.Params {
		if n, ok := p.(json.Number); ok {
			v, err := n.Int64()
			if err != nil {
				h.writeError(w, errs.Wrap(err, errs.Invalid, "server.sql", "numeric parameters must be signed 64-bit integers").With("object", id))
				return
			}
			body.Params[i] = v
		}
	}
	result, err := h.store.Query(r.Context(), id, body.SQL, body.Params)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, result)
}
