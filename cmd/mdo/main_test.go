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
		name, path, reqBody string
		args                []string
	}{
		{"create", "/objects/alpha", "", []string{"create", "alpha"}},
		{"sql", "/objects/alpha/sql", `{"params":[],"sql":"SELECT 1"}`, []string{"sql", "alpha", "SELECT 1"}},
		{"sql params", "/objects/alpha/sql", `{"params":[42,"hi",null,"two words"],"sql":"SELECT ?, ?, ?, ?"}`,
			[]string{"sql", "alpha", "SELECT ?, ?, ?, ?", "42", `"hi"`, "null", "two words"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.Host != "example.test:9999" || r.URL.Path != tc.path {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				var body []byte
				if r.Body != nil {
					body, _ = io.ReadAll(r.Body)
				}
				if got := strings.TrimSpace(string(body)); got != tc.reqBody {
					t.Fatalf("request body: %s; want %s", got, tc.reqBody)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"alpha"}`)), Header: make(http.Header)}, nil
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
			if out.String() != "{\n  \"id\": \"alpha\"\n}\n" {
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
		{name: "bad query", code: 400, body: "bad query: not authorized\n", want: "HTTP 400: bad query: not authorized"},
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
			cmd.SetArgs([]string{"sql", "alpha", "SELECT 1"})
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
	for _, args := range [][]string{{"create"}, {"create", "a", "b"}, {"sql"}, {"sql", "a"}, {"create", "a", "--unknown"}} {
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
