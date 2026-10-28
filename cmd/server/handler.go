package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
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

// writeError maps store errors to HTTP status codes.
func writeError(w http.ResponseWriter, err error) {
	code, msg := http.StatusInternalServerError, err.Error()
	switch {
	case errors.Is(err, store.ErrBadQuery), errors.Is(err, store.ErrInvalidID):
		code = http.StatusBadRequest
	case errors.Is(err, store.ErrNotFound):
		code = http.StatusNotFound
	default:
		msg = "storage error" // Don't leak internal error details.
	}
	http.Error(w, msg, code)
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
	var body struct {
		SQL    string `json:"sql"`
		Params []any  `json:"params"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.UseNumber() // Keep integers exact instead of turning them into float64.
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	for i, p := range body.Params {
		if n, ok := p.(json.Number); ok {
			v, err := n.Int64()
			if err != nil {
				http.Error(w, "numeric parameters must be signed 64-bit integers", http.StatusBadRequest)
				return
			}
			body.Params[i] = v
		}
	}
	result, err := h.store.Query(r.Context(), chi.URLParam(r, "id"), body.SQL, body.Params)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, result)
}
