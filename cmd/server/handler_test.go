package main

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
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

// captureLog sends the log to a buffer until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestWriteErrorKinds(t *testing.T) {
	captureLog(t)
	for _, tc := range []struct {
		kind errs.Kind
		code int
	}{
		{errs.Invalid, 400},
		{errs.BadQuery, 400},
		{errs.NotFound, 404},
		{errs.Canceled, 503},
		{errs.Internal, 500},
	} {
		w := httptest.NewRecorder()
		writeError(w, errs.Wrap(errors.New("cause"), tc.kind, "store.Query", "shown message"))
		if w.Code != tc.code {
			t.Errorf("%v: got %d; want %d", tc.kind, w.Code, tc.code)
		}
		if body := w.Body.String(); !strings.HasPrefix(body, "shown message (log id ") || strings.Contains(body, "cause") {
			t.Errorf("%v: body %q", tc.kind, body)
		}
	}
}

func TestWriteErrorDoesNotLeak(t *testing.T) {
	logs := captureLog(t)
	for _, tc := range []struct {
		err       error
		wantStack bool
	}{
		{errors.New("secret path /x"), false}, // not an *errs.Error: a bug
		{errs.Wrap(errors.New("secret path /x"), errs.Internal, "store.Query", "storage error"), true},
	} {
		logs.Reset()
		w := httptest.NewRecorder()
		writeError(w, tc.err)
		body := w.Body.String()
		_, id, found := strings.Cut(body, "log id ")
		id = strings.TrimSuffix(strings.TrimSpace(id), ")")
		if w.Code != 500 || strings.Contains(body, "secret") || !found {
			t.Errorf("got %d %q", w.Code, body)
		}
		if !strings.Contains(logs.String(), "secret path /x") || !strings.Contains(logs.String(), "["+id+"]") {
			t.Errorf("log missing cause or id %q: %s", id, logs)
		}
		if tc.wantStack && !strings.Contains(logs.String(), "goroutine ") {
			t.Errorf("internal error log has no stack: %s", logs)
		}
	}
}

func TestHandlerErrorsHaveLogID(t *testing.T) {
	captureLog(t)
	h := testObject(t)
	for body, want := range map[string]string{
		`not json`:                          "invalid JSON (log id ",
		`{"sql":"SELECT ?","params":[1.5]}`: "numeric parameters must be signed 64-bit integers (log id ",
	} {
		w := request(h, "POST", "/objects/alpha/sql", body)
		if w.Code != 400 || !strings.HasPrefix(w.Body.String(), want) {
			t.Errorf("%s: got %d %q", body, w.Code, w.Body.String())
		}
	}
}
