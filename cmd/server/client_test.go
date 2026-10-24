package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/nickstrad/mini-durable-objects/internal/apiclient"
)

// Exercise generated request serialization and response parsing against real handlers.
func TestGeneratedClient(t *testing.T) {
	h, db := testObject(t)
	srv := httptest.NewServer(h)
	defer srv.Close()
	client, err := apiclient.NewClientWithResponses(srv.URL, apiclient.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	state, err := client.GetStateWithResponse(ctx, "task-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if state.JSON200 == nil || state.JSON200.Status != apiclient.Pending {
		t.Fatalf("state: %+v", state)
	}
	claim, err := client.ClaimTaskWithResponse(ctx, "task-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if claim.JSON200 == nil || !claim.JSON200.Claimed {
		t.Fatalf("claim: %+v", claim)
	}
	claim, err = client.ClaimTaskWithResponse(ctx, "task-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if claim.JSON200 == nil || claim.JSON200.Claimed {
		t.Fatalf("repeat claim: %+v", claim)
	}
	fail := true
	failed, err := client.FinishTaskWithResponse(ctx, "task-alpha", &apiclient.FinishTaskParams{Fail: &fail})
	if err != nil {
		t.Fatal(err)
	}
	if failed.StatusCode() != 500 {
		t.Fatalf("failed finish: %+v", failed)
	}
	assertStored(t, db, "running", 0)
	finished, err := client.FinishTaskWithResponse(ctx, "task-alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if finished.JSON200 == nil || finished.JSON200.Status != apiclient.Completed {
		t.Fatalf("finish: %+v", finished)
	}
	assertStored(t, db, "completed", 1)
	conflict, err := client.FinishTaskWithResponse(ctx, "task-alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if conflict.StatusCode() != 409 {
		t.Fatalf("repeat finish: %+v", conflict)
	}
	created, err := client.CreateObjectWithResponse(ctx, "task-beta")
	if err != nil {
		t.Fatal(err)
	}
	if created.JSON200 == nil || created.JSON200.Id != "task-beta" || created.JSON200.Status != apiclient.Pending {
		t.Fatalf("create: %+v", created)
	}
	missing, err := client.GetStateWithResponse(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if missing.StatusCode() != 404 {
		t.Fatalf("missing object: %+v", missing)
	}
}
