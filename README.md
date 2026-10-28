# Mini Durable Objects

A small Go learning project with one SQLite database per object.

## Run the API and CLI

Start the server:

```sh
make start   # or: go run ./cmd/server -addr 127.0.0.1:8787 -data data/objects -idle 1m -log-level info -log-format text
```

An object's database stays open while it is in use and closes after `-idle`
(default 1 minute) without requests; the next request reopens it.

The server writes logs to stderr. Use `-log-level` to set the lowest level:
`debug`, `info`, `warn`, or `error`. The default is `info`. Use `-log-format` to
select `text` or `json`. The default is `text`. The `json` format writes one JSON
object on each line. At the `debug` level, the server writes one record for each
HTTP request. [CODING_STYLE.md](CODING_STYLE.md#logs) gives the levels and the keys.

In another terminal, seed objects (alpha, beta, and fake-1..fake-N; N defaults
to 5). Seeding builds `bin/mdo` and runs `mdo create` for each ID. A new object
starts with an empty database:

```sh
make seed        # or: make seed N=10, ./scripts/seed.sh 10
./bin/mdo create gamma
./bin/mdo sql alpha 'CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)'
./bin/mdo sql alpha 'INSERT INTO notes(body) VALUES(?)' hello
./bin/mdo sql alpha 'SELECT * FROM notes WHERE id = ?' 1
```

Each extra argument to `mdo sql` fills one `?` placeholder. An argument that is
valid JSON (`42`, `true`, `null`, `'"quoted"'`) is sent as that value; anything
else is sent as a string.

The CLI prints JSON to stdout. HTTP and connection errors go to stderr and exit
with status 1.

```sh
./bin/mdo sql alpha 'SELECT body FROM notes' | jq -r '.rows[][0]'
./bin/mdo --server http://127.0.0.1:8787 sql alpha 'SELECT 1'
./bin/mdo --help
```

`create` is the only command that makes a new object; `sql` returns 404 for an
unknown ID. Creating an existing object keeps its data.

### What SQL is allowed

Each request runs in its own transaction, so if any statement fails, none of
them stick. SQLite's authorizer checks every action while it prepares the SQL
(see `internal/store/query.go`):

- Allowed: reads, inserts, updates, deletes, and creating, altering, or
  dropping tables and indexes.
- Rejected: `PRAGMA`, `ATTACH`, `BEGIN`/`COMMIT`, triggers, views, virtual
  tables, and temp objects.

Results are limited to 100 rows, and each request has a 2-second time limit.

## API contract and client generation

[api/openapi.yaml](api/openapi.yaml) is the API contract, in OpenAPI 3.0 format
(the modern Swagger specification). It can be opened in Swagger Editor or other
OpenAPI tools. The routes are:

| CLI | HTTP operation |
| --- | --- |
| `mdo create ID` | `POST /objects/{id}` |
| `mdo sql ID SQL [PARAM...]` | `POST /objects/{id}/sql` |

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
exercise requests and typed responses against the real HTTP handlers. Do not
edit `client.gen.go` by hand. To upgrade the generator deliberately, run:

```sh
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@VERSION
make generate
make check
```

The `mdo` commands communicate with the running HTTP server.
