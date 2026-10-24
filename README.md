# Mini Durable Objects

A small Go learning project with one SQLite database per object.

## Run the API and CLI

Start the server:

```sh
make start
```

In another terminal, seed objects (task-alpha, task-beta, and fake-1..fake-N;
N defaults to 5). Seeding builds `bin/mdo` and runs `mdo create` for each ID.
It also adds the events table to existing databases:

```sh
make seed        # or: make seed N=10, ./scripts/seed.sh 10
./bin/mdo create task-gamma
./bin/mdo state task-alpha
./bin/mdo claim task-alpha
./bin/mdo finish task-alpha
```

The CLI prints JSON to stdout. HTTP and connection errors go to stderr and exit
with status 1. A claim that loses the race returns `claimed: false` and exits
successfully, matching the API.

```sh
./bin/mdo state task-alpha | jq -r '.status'
./bin/mdo --server http://127.0.0.1:8787 state task-alpha
./bin/mdo --help
```

To try rollback, claim a pending task, then run:

```sh
./bin/mdo finish task-alpha --fail
./bin/mdo state task-alpha   # still running
./bin/mdo finish task-alpha  # commits completion and its event
```

`create` is the only command that makes a new object; the others return 404 for
an unknown ID. Use a fresh ID for a new pending task; creating an existing
object preserves its status.

## API contract and client generation

[api/openapi.yaml](api/openapi.yaml) is the API contract, in OpenAPI 3.0 format
(the modern Swagger specification). It can be opened in Swagger Editor or other
OpenAPI tools. The routes are:

| CLI | HTTP operation |
| --- | --- |
| `mdo create ID` | `POST /objects/{id}` |
| `mdo state ID` | `GET /objects/{id}` |
| `mdo claim ID` | `POST /objects/{id}/claim` |
| `mdo finish ID [--fail]` | `POST /objects/{id}/finish[?fail=true]` |

The Go client is generated into `internal/apiclient/client.gen.go` using
[api/client.yaml](api/client.yaml). The generator is pinned as a Go tool in
`go.mod`; no separate generator install, Node, Java, or Docker is needed. The
first run needs network access to download Go dependencies.

When changing the API:

1. Update `api/openapi.yaml` and the handlers in `cmd/server/handler.go` together.
   Keep existing `operationId` values stable unless intentionally renaming client
   methods.
2. Run `make generate` (or `./scripts/generate-clients.sh`).
3. Update command mappings in `cmd/mdo/main.go` for new operations or flags.
   The CLI uses the generated client; commands are maintained by hand.
4. Update handler, generated-client integration, and CLI tests as needed.
5. Run `make check` and `make build-cli`.
6. Commit the spec, generator configuration, generated client, CLI, and tests
   together, including `go.mod` and `go.sum` when dependencies change.

`make check` regenerates to a temporary file and fails if the checked-in client
is stale, then runs gofmt, vet, and tests. Generated-client integration tests
exercise requests and typed responses against the real HTTP handlers. Handler
tests also verify concurrent claims and finish rollback. Do not edit `client.gen.go` by
hand. To upgrade the generator deliberately, run:

```sh
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@VERSION
make generate
make check
```

The `mdo` commands communicate with the running HTTP server.
