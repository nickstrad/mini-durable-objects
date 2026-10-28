package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nickstrad/mini-durable-objects/internal/errs"
	"github.com/nickstrad/mini-durable-objects/internal/store"
)

type Object struct {
	ID string `json:"id"`
}

// handler maps HTTP requests to Store calls. It passes /sql requests through
// without inspecting the SQL; the store decides what is allowed.
type handler struct{ store store.Store }

func newRouter(s store.Store) http.Handler {
	h := handler{store: s}
	mux := chi.NewRouter()
	mux.Post("/objects/{id}", h.create)
	mux.Post("/objects/{id}/sql", h.sql)
	return mux
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

// writeError is the top of the server. It logs the error once with a log ID and
// shows the client only the Message and that ID.
func writeError(w http.ResponseWriter, err error) {
	id := rand.Text()

	e, ok := errors.AsType[*errs.Error](err)
	if !ok {
		log.Printf("[%s] BUG: unwrapped error reached the top: %v", id, err)
		http.Error(w, "internal error (log id "+id+")", http.StatusInternalServerError)
		return
	}
	// Only Internal errors carry a stack (see errs.Wrap).
	log.Printf("[%s] %v: %v %v\n%s", id, e.Kind, e, e.Misc, e.Stack)
	code := http.StatusInternalServerError
	switch e.Kind {
	case errs.Invalid, errs.BadQuery:
		code = http.StatusBadRequest
	case errs.NotFound:
		code = http.StatusNotFound
	case errs.Canceled:
		code = http.StatusServiceUnavailable
	}
	http.Error(w, e.Message+" (log id "+id+")", code)
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Create(r.Context(), id); err != nil {
		writeError(w, err)
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
		writeError(w, errs.Wrap(err, errs.Invalid, "server.sql", "invalid JSON").With("object", id))
		return
	}
	for i, p := range body.Params {
		if n, ok := p.(json.Number); ok {
			v, err := n.Int64()
			if err != nil {
				writeError(w, errs.Wrap(err, errs.Invalid, "server.sql", "numeric parameters must be signed 64-bit integers").With("object", id))
				return
			}
			body.Params[i] = v
		}
	}
	result, err := h.store.Query(r.Context(), id, body.SQL, body.Params)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, result)
}
