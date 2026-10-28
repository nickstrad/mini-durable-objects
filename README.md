# Mini Durable Objects

A small runtime on one machine, similar to Cloudflare Durable Objects. Each object has
an ID and its own SQLite database. The requests for one object run one at a time on the
goroutine of that object. The requests for different objects run in parallel. An HTTP
API and a CLI (`mdo`) give access to the objects.

```
HTTP client or mdo -> router -> store -> registry -> object goroutine -> SQLite file
                                                     (one for each object)
```

## Requirements

- Go 1.26 or later.
- A C compiler. The SQLite driver, `mattn/go-sqlite3`, uses cgo. On macOS, install
  the Xcode Command Line Tools.
- Optional: the DuckDB CLI, version 2.0 or later, to analyze events.
- Optional: `jq`, to read the JSON output of the CLI.

The first build needs network access to download the Go modules.

## Start the server

```sh
make start                                    # go run ./cmd/server
make start ARGS="-events data/events -log-level debug"
make build && ./bin/server -idle 30s          # a built binary
```

The server listens on `127.0.0.1:8787`. To stop it, press Ctrl+C or send SIGTERM. The
server then lets the open requests finish, writes the events, and stops with exit
status 0.

`go run` starts the server as a child process. Thus, to stop the server with `kill`,
use `./bin/server`, not `make start`, and send the signal to that process.

| Flag | Default | What it does |
|---|---|---|
| `-addr` | `127.0.0.1:8787` | The address that the server listens on. |
| `-data` | `data/objects` | The directory for the object databases. The path is relative to the current directory. |
| `-idle` | `1m` | The server closes the database of an object after the object is idle for this time. The next request opens it again. |
| `-log-level` | `info` | The lowest log level: `debug`, `info`, `warn`, or `error`. At `debug`, the server writes one record for each HTTP request. |
| `-log-format` | `text` | `text`, or `json` for one JSON object on each line. |
| `-events` | empty | A directory for event CSV files. Empty turns events off. See [Events](#events). |
| `-events-flush` | `1s` | The time between two flushes of the event files. |

The server writes its logs to stderr. [CODING_STYLE.md](CODING_STYLE.md#logs) gives the
levels and the keys. If a flag is not correct, the server stops with exit status 2. If
the HTTP server fails, the server stops with exit status 1.

## Use the CLI

In a different terminal, make some objects. `make seed` builds `bin/mdo` and makes the
objects `alpha`, `beta`, and `fake-1` to `fake-N`. N is 5 if you do not set it. A new
object has an empty database.

```sh
make seed                                     # or: make seed N=10
./bin/mdo create gamma
./bin/mdo sql alpha 'CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)'
./bin/mdo sql alpha 'INSERT INTO notes(body) VALUES(?)' hello
./bin/mdo sql alpha 'SELECT * FROM notes WHERE id = ?' 1
./bin/mdo sql alpha 'SELECT body FROM notes' | jq -r '.rows[][0]'
./bin/mdo --server http://127.0.0.1:8787 sql alpha 'SELECT 1'
./bin/mdo --help
```

- Each argument after the SQL fills one `?` placeholder. If an argument is valid JSON,
  for example `42`, `true`, `null`, or `'"quoted"'`, the CLI sends that JSON value.
  The CLI sends all other arguments as strings.
- The CLI writes the JSON result to stdout.
- If there is an error, the CLI writes it to stderr and stops with exit status 1.
- Only `create` makes a new object. For an unknown ID, `sql` gives HTTP 404.
- If you create an object that exists, its data does not change.

### Errors

An error response from the server is text. It contains the message and a log ID, for
example `bad query: no such table: nope (log id CI5KFMF3NFWWBHJNSTU7OKKUIY)`. Use the
log ID to find the full error in the server log. The server shows the client only the
message. The cause and the stack trace stay in the log.
[CODING_STYLE.md](CODING_STYLE.md#errors) gives the rules for errors.

### The SQL that the server permits

Each request runs in its own transaction. If one statement fails, the server rolls
back all statements of the request. SQLite calls an authorizer for each action while it
prepares the SQL. The authorizer is in `internal/store/query.go`.

- Permitted: reads, inserts, updates, deletes, and the creation, change, and removal of
  tables and indexes.
- Not permitted: `PRAGMA`, `ATTACH`, `BEGIN`, `COMMIT`, triggers, views, virtual tables,
  and temporary objects.

A result can have 100 rows or fewer. Each request can run for 2 seconds or less.

## Events

Events are off by default. With `-events DIR`, the server writes CSV files to DIR:

- One row for each request, with the time at each step: the registry lock, the open of
  the database, the wait in the object inbox, and SQLite.
- One row for each activation or eviction of an object.
- One row for the run, when the server stops correctly.

If the server stops without SIGINT or SIGTERM, the events of the last flush interval
are lost. [CODING_STYLE.md](CODING_STYLE.md#events) gives the columns and the rules
that the events must obey.

To analyze the events, install the DuckDB CLI and run `make events`. It loads the CSV
files and runs the queries in `scripts/events/`.

```sh
make start ARGS="-events data/events"         # record
make events                                   # all queries on data/events
make events Q=contention                      # one query
make events Q="latency invariants" DIR=/tmp/run1
./scripts/events.sh -d data/events summary    # the same, without make
```

| Query | What it shows |
|---|---|
| `summary` | Requests, errors, and the quality of the event data, for each run. |
| `latency` | Percentiles of the request time for cold and warm requests, and the share of each step. |
| `contention` | The wait for each object and for the registry lock, and the wait for each queue depth. |
| `lifecycle` | Activations and evictions. |
| `timeline` | Requests, errors, and latency for each second. |
| `invariants` | The number of violations of each rule in CODING_STYLE.md. 0 is correct. |

## Make targets

| Target | What it does |
|---|---|
| `make start` | Runs the server with `go run`. `ARGS` gives more flags. |
| `make build` | Builds `bin/server` and `bin/mdo`. `make build-server` and `make build-cli` build one of them. |
| `make seed` | Builds `bin/mdo` and makes the seed objects. The server must run. `N` sets the number of fake objects. |
| `make events` | Runs the DuckDB queries. `Q` selects the queries, and `DIR` selects the events directory. |
| `make test` | Runs all tests in a random order. |
| `make race` | Runs all tests with the race detector. |
| `make check` | Makes sure that the generated client is current, formats the code, runs `go vet`, and runs the tests. Run it before you finish a change. |
| `make generate` | Generates the API client again from `api/openapi.yaml`. |
| `make clean` | Removes `bin/`. It does not remove `data/`. |

## Project layout

| Path | What it contains |
|---|---|
| `cmd/server` | The HTTP server: flags, logger, router, and handlers. |
| `cmd/mdo` | The CLI. It uses the generated API client. |
| `internal/store` | The `Store` interface and its SQLite adapter. The registry runs one goroutine for each active object and closes idle objects. |
| `internal/errs` | The error type that goes across package boundaries. |
| `internal/events` | The event recorder. It writes the CSV files on its own goroutine. |
| `internal/apiclient` | The generated API client. Do not edit it. |
| `api/` | The OpenAPI contract and the configuration of the generator. |
| `scripts/` | The seed script, the generator script, and the DuckDB queries. |
| `agent_work/validation/` | Scripts that test the tests, for example mutation testing. `index.md` lists them. |
| `CODING_STYLE.md` | The rules for errors, logs, events, and docs. |
| `AGENTS.md` | The instructions for coding agents. |

## API contract and client generation

[api/openapi.yaml](api/openapi.yaml) is the API contract, in OpenAPI 3.0 format. You can
open it in Swagger Editor or in a different OpenAPI tool. These are the routes:

| CLI | HTTP operation |
| --- | --- |
| `mdo create ID` | `POST /objects/{id}` |
| `mdo sql ID SQL [PARAM...]` | `POST /objects/{id}/sql` |

The generator writes the Go client to `internal/apiclient/client.gen.go`. Its
configuration is [api/client.yaml](api/client.yaml). `go.mod` pins the generator as a Go
tool. Thus, you do not need to install a generator, Node, Java, or Docker.

To change the API, do these steps:

1. Change `api/openapi.yaml` and the handlers in `cmd/server/handler.go` together. Do
   not change an `operationId` unless you want to change the name of a client method.
2. Run `make generate`.
3. If you add an operation or a flag, change the commands in `cmd/mdo/main.go`. The CLI
   uses the generated client, but a person writes the commands.
4. Change the handler tests, the tests of the generated client, and the CLI tests.
5. Run `make check` and `make build`.
6. Commit these files together: the spec, the generator configuration, the generated
   client, the CLI, and the tests. If the dependencies changed, also commit `go.mod`
   and `go.sum`.

`make check` generates the client into a temporary file. If the client in the
repository is different, `make check` fails. The tests of the generated client send
requests to the real HTTP handlers and read the typed responses. Do not edit
`client.gen.go`.

To change the version of the generator, run these commands:

```sh
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@VERSION
make generate
make check
```
