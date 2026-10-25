package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nickstrad/mini-durable-objects/internal/store"
)

func testObject(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	h := newRouter(store.NewSQLite(dir))
	if w := request(h, "POST", "/objects/task-alpha"); w.Code != 200 {
		t.Fatalf("create: got %d: %s", w.Code, w.Body.String())
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "task-alpha", "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return h, db
}

func request(h http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func assertStored(t *testing.T, db *sql.DB, wantStatus string, wantEvents int) {
	t.Helper()
	var status string
	var events int
	if err := db.QueryRow("SELECT status FROM task WHERE id=1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || events != wantEvents {
		t.Fatalf("stored status=%q events=%d; want %q %d", status, events, wantStatus, wantEvents)
	}
}

func TestTaskLifecycle(t *testing.T) {
	h, db := testObject(t)
	for _, step := range []struct {
		method, path string
		code         int
		status       string
		events       int
	}{
		{"GET", "/objects/task-alpha", 200, "pending", 0},
		{"POST", "/objects/task-alpha/finish", 409, "pending", 0},
		{"POST", "/objects/task-alpha/claim", 200, "running", 0},
		{"POST", "/objects/task-alpha/finish?fail=true", 500, "running", 0},
		{"POST", "/objects/task-alpha/finish", 200, "completed", 1},
		{"POST", "/objects/task-alpha/finish", 409, "completed", 1},
	} {
		w := request(h, step.method, step.path)
		if w.Code != step.code {
			t.Fatalf("%s %s: got %d: %s", step.method, step.path, w.Code, w.Body.String())
		}
		assertStored(t, db, step.status, step.events)
	}
	w := request(h, "GET", "/objects/task-alpha")
	var state ObjectState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.ID != "task-alpha" || state.Status != "completed" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestConcurrentClaims(t *testing.T) {
	h, db := testObject(t)
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- request(h, "POST", "/objects/task-alpha/claim")
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for w := range results {
		if w.Code != 200 {
			t.Fatalf("claim: %d %s", w.Code, w.Body.String())
		}
		var result struct {
			Claimed bool `json:"claimed"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Claimed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("got %d successful claims; want 1", winners)
	}
	assertStored(t, db, "running", 0)
}

func TestRouteErrors(t *testing.T) {
	h, db := testObject(t)
	for _, route := range []struct{ method, action string }{{"GET", ""}, {"POST", "/claim"}, {"POST", "/finish"}} {
		for _, tc := range []struct {
			id   string
			code int
		}{{"missing", 404}, {"INVALID", 400}} {
			w := request(h, route.method, "/objects/"+tc.id+route.action)
			if w.Code != tc.code {
				t.Fatalf("%s: got %d; want %d", tc.id+route.action, w.Code, tc.code)
			}
		}
	}
	if w := request(h, "GET", "/objects/task-alpha/claim"); w.Code != 405 {
		t.Fatalf("GET claim: got %d", w.Code)
	}
	assertStored(t, db, "pending", 0)
}

func TestCreateKeepsExistingStatus(t *testing.T) {
	h, db := testObject(t)
	request(h, "POST", "/objects/task-alpha/claim")
	w := request(h, "POST", "/objects/task-alpha")
	var state ObjectState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || state.Status != "running" {
		t.Fatalf("create existing: %d %+v", w.Code, state)
	}
	if w := request(h, "POST", "/objects/INVALID"); w.Code != 400 {
		t.Fatalf("create invalid: got %d", w.Code)
	}
	assertStored(t, db, "running", 0)
}
