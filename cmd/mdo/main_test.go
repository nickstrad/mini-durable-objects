package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCommands(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, query, body string
		args                            []string
	}{
		{"state", "GET", "/objects/task-alpha", "", `{"id":"task-alpha","status":"pending"}`, []string{"state", "task-alpha"}},
		{"create", "POST", "/objects/task-alpha", "", `{"id":"task-alpha","status":"pending"}`, []string{"create", "task-alpha"}},
		{"claim", "POST", "/objects/task-alpha/claim", "", `{"id":"task-alpha","claimed":true}`, []string{"claim", "task-alpha"}},
		{"claim loses", "POST", "/objects/task-alpha/claim", "", `{"id":"task-alpha","claimed":false}`, []string{"claim", "task-alpha"}},
		{"finish", "POST", "/objects/task-alpha/finish", "", `{"id":"task-alpha","status":"completed"}`, []string{"finish", "task-alpha"}},
		{"finish failure flag", "POST", "/objects/task-alpha/finish", "fail=true", `{"id":"task-alpha","status":"completed"}`, []string{"finish", "task-alpha", "--fail"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != tc.method || r.URL.Host != "example.test:9999" || r.URL.Path != tc.path || r.URL.RawQuery != tc.query {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			cmd := newRootCommand(client)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(append([]string{"--server", "http://example.test:9999"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("got %d requests", calls)
			}
			if !strings.Contains(out.String(), "\"id\": \"task-alpha\"") {
				t.Fatalf("output: %s", &out)
			}
		})
	}
}

func TestCommandFailures(t *testing.T) {
	for _, tc := range []struct {
		name         string
		code         int
		body, want   string
		transportErr error
	}{
		{name: "missing", code: 404, body: "object not found\n", want: "HTTP 404: object not found"},
		{name: "conflict", code: 409, body: "task is not running\n", want: "HTTP 409: task is not running"},
		{name: "rollback", code: 500, body: "simulated failure before commit\n", want: "HTTP 500: simulated failure before commit"},
		{name: "bad json", code: 200, body: "not json", want: "invalid API JSON"},
		{name: "network", want: "offline", transportErr: errors.New("offline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				if tc.transportErr != nil {
					return nil, tc.transportErr
				}
				return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			cmd := newRootCommand(client)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"finish", "task-alpha", "--fail"})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error: %v; want %q", err, tc.want)
			}
			if out.Len() != 0 {
				t.Fatalf("unexpected stdout: %s", &out)
			}
		})
	}
}

func TestArgumentsCheckedBeforeRequest(t *testing.T) {
	for _, args := range [][]string{{"state"}, {"create"}, {"claim", "a", "b"}, {"finish"}, {"state", "a", "--fail"}} {
		cmd := newRootCommand(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("unexpected request")
			return nil, nil
		})})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}
