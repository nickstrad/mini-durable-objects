package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nickstrad/mini-durable-objects/internal/store"
)

type ObjectState struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// handler maps HTTP requests to Store calls; it knows nothing about SQL.
type handler struct{ store store.Store }

func newRouter(s store.Store) http.Handler {
	h := handler{store: s}
	mux := chi.NewRouter()
	mux.Get("/objects/{id}", h.state)
	mux.Post("/objects/{id}", h.create)
	mux.Post("/objects/{id}/claim", h.claim)
	mux.Post("/objects/{id}/finish", h.finish)
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
	case errors.Is(err, store.ErrInvalidID):
		code = http.StatusBadRequest
	case errors.Is(err, store.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, store.ErrNotRunning):
		code = http.StatusConflict
	case errors.Is(err, store.ErrSimulatedFailure): // 500, keeps its message for the demo
	default:
		msg = "storage error" // Don't leak internal error details.
	}
	http.Error(w, msg, code)
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.store.Create(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, ObjectState{ID: id, Status: status})
}

func (h handler) state(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.store.State(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, ObjectState{ID: id, Status: status})
}

func (h handler) claim(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claimed, err := h.store.Claim(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, struct {
		ID      string `json:"id"`
		Claimed bool   `json:"claimed"`
	}{ID: id, Claimed: claimed})
}

func (h handler) finish(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// ?fail=true keeps the rollback experiment available over HTTP.
	if err := h.store.Finish(r.Context(), id, r.URL.Query().Get("fail") == "true"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, ObjectState{ID: id, Status: "completed"})
}
