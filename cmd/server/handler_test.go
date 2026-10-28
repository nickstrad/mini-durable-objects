package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/store"
)

// testObject returns a router whose store holds one object, "alpha", with a
// notes table.
func testObject(t *testing.T) http.Handler {
	t.Helper()
	h := newRouter(store.NewSQLite(t.TempDir(), time.Minute))
	if w := request(h, "POST", "/objects/alpha", ""); w.Code != 200 {
		t.Fatalf("create: got %d: %s", w.Code, w.Body.String())
	}
	if w := request(h, "POST", "/objects/alpha/sql", `{"sql":"CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)"}`); w.Code != 200 {
		t.Fatalf("create table: got %d: %s", w.Code, w.Body.String())
	}
	return h
}

func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestCreate(t *testing.T) {
	h := testObject(t)
	request(h, "POST", "/objects/alpha/sql", `{"sql":"INSERT INTO notes(body) VALUES('kept')"}`)
	// Creating an existing object keeps its data.
	if w := request(h, "POST", "/objects/alpha", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"id":"alpha"}` {
		t.Fatalf("create existing: got %d: %s", w.Code, w.Body.String())
	}
	if w := request(h, "POST", "/objects/alpha/sql", `{"sql":"SELECT body FROM notes"}`); !strings.Contains(w.Body.String(), `"kept"`) {
		t.Fatalf("data lost: %s", w.Body.String())
	}
}

func TestRouteErrors(t *testing.T) {
	h := testObject(t)
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{"POST", "/objects/INVALID", 400},
		{"POST", "/objects/INVALID/sql", 400},
		{"POST", "/objects/missing/sql", 404},
		{"GET", "/objects/alpha", 405},
		{"GET", "/objects/alpha/sql", 405},
	} {
		if w := request(h, tc.method, tc.path, `{"sql":"SELECT 1"}`); w.Code != tc.code {
			t.Errorf("%s %s: got %d; want %d", tc.method, tc.path, w.Code, tc.code)
		}
	}
}

func TestSQLRoute(t *testing.T) {
	h := testObject(t)
	run := func(body string) *httptest.ResponseRecorder {
		return request(h, "POST", "/objects/alpha/sql", body)
	}

	if w := run(`{"sql":"INSERT INTO notes(body) VALUES(?)","params":["note"]}`); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"columns":[],"rows":[]}` {
		t.Fatalf("insert: got %d: %s", w.Code, w.Body.String())
	}
	w := run(`{"sql":"SELECT id, body FROM notes WHERE id = ?","params":[1]}`)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"columns":["id","body"],"rows":[[1,"note"]]}` {
		t.Fatalf("select: got %d: %s", w.Code, w.Body.String())
	}

	// Clients manage their own tables and indexes.
	for _, body := range []string{
		`{"sql":"CREATE TABLE scratch (x)"}`,
		`{"sql":"CREATE INDEX scratch_x ON scratch(x)"}`,
		`{"sql":"ALTER TABLE scratch ADD COLUMN y"}`,
		`{"sql":"DROP INDEX scratch_x"}`,
		`{"sql":"DROP TABLE scratch"}`,
	} {
		if w := run(body); w.Code != 200 {
			t.Errorf("%s: got %d: %s", body, w.Code, w.Body.String())
		}
	}

	for _, body := range []string{
		`not json`,
		// The authorizer rejects these while SQLite prepares them.
		`{"sql":"PRAGMA query_only=0"}`,
		`{"sql":"ATTACH DATABASE '/tmp/other.db' AS other"}`,
		`{"sql":"COMMIT"}`,
		`{"sql":"CREATE TEMP TABLE t(x)"}`,
		`{"sql":"CREATE VIEW v AS SELECT 1"}`,
		`{"sql":"CREATE TRIGGER tr AFTER INSERT ON notes BEGIN DELETE FROM notes; END"}`,
		// SQLite itself refuses this one.
		`{"sql":"UPDATE sqlite_master SET sql=''"}`,
		`{"sql":"SELECT * FROM missing_table"}`,
		`{"sql":"SELECT ?","params":[1.5]}`,
		`{"sql":"SELECT i FROM (WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n) SELECT i FROM n LIMIT 101)"}`,
	} {
		if w := run(body); w.Code != 400 {
			t.Errorf("%s: got %d: %s", body, w.Code, w.Body.String())
		}
	}
	// A rejected statement after an allowed one rolls both back.
	if w := run(`{"sql":"INSERT INTO notes(body) VALUES('x'); PRAGMA query_only=0"}`); w.Code != 400 {
		t.Fatalf("multi: got %d", w.Code)
	}
	// SQLite parses the SQL, so ';' inside a string is fine.
	w = run(`{"sql":"WITH n(i) AS (SELECT 1) SELECT i, 'a;b' FROM n, notes"}`)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"columns":["i","'a;b'"],"rows":[[1,"a;b"]]}` {
		t.Fatalf("with: got %d: %s", w.Code, w.Body.String())
	}
	if w := run(`{"sql":"SELECT 1 FROM notes WHERE 0"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"rows":[]`) {
		t.Fatalf("empty select: got %d: %s", w.Code, w.Body.String())
	}
}
