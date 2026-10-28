# mini-durable-objects

A small single-machine runtime inspired by Cloudflare Durable Objects. Each object has its own
SQLite database, and requests for one object run one at a time on that object's goroutine.
Clients reach an object over HTTP: `POST /objects/{id}` creates it, `POST /objects/{id}/sql`
runs SQL against it, limited by SQLite's authorizer (`internal/store/query.go`).

```
HTTP Client / Coding Agent -> Object Router -> resolve object ID -> Object A / Object B
                                                                    -> SQLite A / SQLite B
```

Language: Go. This is a separate project from `../durable-actor`; do not share its code or
assume its conventions.

Changes ship with tests or a runnable demo showing them working.

## Go conventions

- Run `make check` (gofmt, `go vet`, and `make test`) before finishing any change.
- Prefer the standard library; add a dependency only when it earns its place (current ones:
  mattn/go-sqlite3, which needs cgo; chi; cobra; oapi-codegen).

## Validation and scripts

- Before validating or testing a change, read `agent_work/validation/index.md` and reuse the
  scripts there (e.g. mutation testing, flakiness loops) instead of writing a custom one.
- Put one-off scripts in `agent_work/validation/` too, and add a line for each to `index.md`:
  what it does and the exact command to run it.
- Scripting languages: Go for anything non-trivial, bash for basic terminal work (copying,
  looping over `go test`). No Python.
- Scripts must work on a temporary copy of the repo, never mutate the working tree, and still
  pass `make check`.
