package main

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/nickstrad/mini-durable-objects/internal/apiclient"
)

// Exercise generated request serialization and response parsing against real handlers.
func TestGeneratedClient(t *testing.T) {
	srv := httptest.NewServer(testObject(t))
	defer srv.Close()
	client, err := apiclient.NewClientWithResponses(srv.URL, apiclient.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	created, err := client.CreateObjectWithResponse(ctx, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if created.JSON200 == nil || created.JSON200.Id != "beta" {
		t.Fatalf("create: %+v", created)
	}

	params := []any{"hi"}
	inserted, err := client.RunSQLWithResponse(ctx, "alpha", apiclient.SQLRequest{Sql: "INSERT INTO notes(body) VALUES(?)", Params: &params})
	if err != nil {
		t.Fatal(err)
	}
	if inserted.StatusCode() != 200 {
		t.Fatalf("insert: %d %s", inserted.StatusCode(), inserted.Body)
	}
	selected, err := client.RunSQLWithResponse(ctx, "alpha", apiclient.SQLRequest{Sql: "SELECT id, body FROM notes"})
	if err != nil {
		t.Fatal(err)
	}
	want := apiclient.SQLResult{Columns: []string{"id", "body"}, Rows: [][]any{{float64(1), "hi"}}}
	if selected.JSON200 == nil || !reflect.DeepEqual(*selected.JSON200, want) {
		t.Fatalf("select: %+v", selected.JSON200)
	}

	denied, err := client.RunSQLWithResponse(ctx, "alpha", apiclient.SQLRequest{Sql: "PRAGMA query_only=0"})
	if err != nil {
		t.Fatal(err)
	}
	if denied.StatusCode() != 400 {
		t.Fatalf("pragma: %d", denied.StatusCode())
	}
	missing, err := client.RunSQLWithResponse(ctx, "missing", apiclient.SQLRequest{Sql: "SELECT 1"})
	if err != nil {
		t.Fatal(err)
	}
	if missing.StatusCode() != 404 {
		t.Fatalf("missing object: %d", missing.StatusCode())
	}
}
