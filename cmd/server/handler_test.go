package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/errs"
	"github.com/nickstrad/mini-durable-objects/internal/store"
)

// discard drops all log records.
var discard = slog.New(slog.DiscardHandler)

// testObject returns a router whose store holds one object, "alpha", with a
// notes table.
func testObject(t *testing.T) http.Handler {
	t.Helper()
	h := newRouter(store.NewSQLite(t.TempDir(), time.Minute, discard, nil), discard)
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

// captureLog returns a logger that writes JSON records to a buffer.
func captureLog(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})), &buf
}

// records decodes the JSON log lines in buf.
func records(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad log line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func TestWriteErrorKinds(t *testing.T) {
	for _, tc := range []struct {
		kind  errs.Kind
		code  int
		level string
	}{
		{errs.Invalid, 400, "INFO"},
		{errs.BadQuery, 400, "INFO"},
		{errs.NotFound, 404, "INFO"},
		{errs.Canceled, 503, "WARN"},
		{errs.Internal, 500, "ERROR"},
	} {
		logger, buf := captureLog(slog.LevelDebug)
		w := httptest.NewRecorder()
		err := errs.Wrap(errors.New("cause"), tc.kind, "store.Query", "shown message").With("object", "alpha")
		handler{logger: logger}.writeError(w, err)
		if w.Code != tc.code {
			t.Errorf("%v: got %d; want %d", tc.kind, w.Code, tc.code)
		}
		body := w.Body.String()
		if !strings.HasPrefix(body, "shown message (log id ") || strings.Contains(body, "cause") {
			t.Errorf("%v: body %q", tc.kind, body)
		}
		recs := records(t, buf)
		if len(recs) != 1 {
			t.Fatalf("%v: got %d records; want 1", tc.kind, len(recs))
		}
		rec := recs[0]
		if rec["level"] != tc.level || rec["msg"] != "request failed" || rec["kind"] != tc.kind.String() ||
			rec["op"] != "store.Query" || rec["object"] != "alpha" {
			t.Errorf("%v: record %v", tc.kind, rec)
		}
		_, hasStack := rec["stack"]
		if hasStack != (tc.kind == errs.Internal) {
			t.Errorf("%v: stack present = %v", tc.kind, hasStack)
		}
	}
}

func TestWriteErrorDoesNotLeak(t *testing.T) {
	for _, tc := range []struct {
		err   error
		msg   string
		level string
	}{
		{errors.New("secret path /x"), "unwrapped error reached the top", "ERROR"},
		{errs.Wrap(errors.New("secret path /x"), errs.Internal, "store.Query", "storage error"), "request failed", "ERROR"},
	} {
		logger, buf := captureLog(slog.LevelInfo)
		w := httptest.NewRecorder()
		handler{logger: logger}.writeError(w, tc.err)
		body := w.Body.String()
		_, id, found := strings.Cut(body, "log id ")
		id = strings.TrimSuffix(strings.TrimSpace(id), ")")
		if w.Code != 500 || strings.Contains(body, "secret") || !found {
			t.Errorf("got %d %q", w.Code, body)
		}
		rec := records(t, buf)[0]
		if rec["log_id"] != id || rec["msg"] != tc.msg || rec["level"] != tc.level {
			t.Errorf("record %v; want log_id %q", rec, id)
		}
		if !strings.Contains(rec["err"].(string), "secret path /x") {
			t.Errorf("log missing cause: %v", rec)
		}
	}
}

func TestHandlerErrorsHaveLogID(t *testing.T) {
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

func TestRequestRecord(t *testing.T) {
	logger, buf := captureLog(slog.LevelDebug)
	h := newRouter(store.NewSQLite(t.TempDir(), time.Minute, logger, nil), logger)
	request(h, "POST", "/objects/alpha", "")
	buf.Reset()
	request(h, "POST", "/objects/alpha/sql", `{"sql":"SELECT 'topsecret-sql'","params":["topsecret-param"]}`)
	request(h, "POST", "/objects/missing/sql", `{"sql":"SELECT 1"}`)

	var reqs []map[string]any
	for _, rec := range records(t, buf) {
		if rec["msg"] == "request" {
			reqs = append(reqs, rec)
		}
	}
	if len(reqs) != 2 {
		t.Fatalf("got %d request records; want 2: %s", len(reqs), buf)
	}
	for i, want := range []struct {
		path   string
		status float64
	}{{"/objects/alpha/sql", 200}, {"/objects/missing/sql", 404}} {
		rec := reqs[i]
		if rec["level"] != "DEBUG" || rec["method"] != "POST" || rec["path"] != want.path || rec["status"] != want.status {
			t.Errorf("record %d: %v", i, rec)
		}
		if _, ok := rec["duration"]; !ok {
			t.Errorf("record %d has no duration", i)
		}
	}
	if strings.Contains(buf.String(), "topsecret") {
		t.Errorf("log has request data: %s", buf)
	}
}

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger, err := newLogger("text", "info", &buf)
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("hidden")
	logger.Info("shown", "object", "alpha")
	if out := buf.String(); strings.Contains(out, "hidden") || !strings.Contains(out, "level=INFO msg=shown object=alpha") {
		t.Errorf("text output: %q", out)
	}

	buf.Reset()
	logger, err = newLogger("json", "debug", &buf)
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("shown", "object", "alpha")
	if rec := records(t, &buf)[0]; rec["level"] != "DEBUG" || rec["msg"] != "shown" || rec["object"] != "alpha" {
		t.Errorf("json record: %v", rec)
	}

	for _, tc := range [][2]string{{"xml", "info"}, {"text", "loud"}, {"text", ""}} {
		if _, err := newLogger(tc[0], tc[1], &buf); err == nil {
			t.Errorf("newLogger(%q, %q): want error", tc[0], tc[1])
		}
	}
}
