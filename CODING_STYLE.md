# Coding style

Write the docs in this repo in ASD-STE100 Simplified Technical English. Use the skill in
`.claude/skills/asd-ste100/`.

## Errors

The error method in this repo comes from *Concurrency in Go* by Katherine Cox-Buday
(O'Reilly, chapter 5). An error is data that moves across boundaries. Each boundary
decides which data moves on.

A good error gives four items:

1. **What happened.** The caller can act on this text, for example "object not found".
2. **Where and when it happened.** The error has the operation name. An `Internal` error
   also has a stack trace. The log line has the time.
3. **A message for the user.** This message is safe to show. It is separate from the
   internal details.
4. **A log ID.** The user can give this ID to the operator. The ID points to the full
   error in the server log.

### The error shape

Each error that goes out of a package is an `*errs.Error`, or it contains an
`*errs.Error`. The code is in `internal/errs/errs.go`. It uses only the standard library.

```go
type Error struct {
	Kind    Kind           // Internal, Invalid, NotFound, BadQuery, or Canceled
	Op      string         // the operation: "store.Query", "server.sql"
	Message string         // safe to show to the client
	Inner   error          // the cause; it can contain internal details
	Stack   []byte         // Internal only; recorded one time, at the first wrap
	Misc    map[string]any // data for debug, for example {"object": id}
}
```

The package has these functions:

| Function | What it does |
|---|---|
| `Wrap(err, kind, op, message)` | Makes `err` into an `*Error`. `err` can be nil. If `err` contains a stack, `Wrap` keeps that stack, because the first stack is the nearest to the cause. If not, `Wrap` records a stack only for `Internal`. Nobody reads the stack of a client error. |
| `(*Error).With(key, value)` | Adds one value to `Misc`. It returns the same `*Error`. |
| `KindOf(err)` | Gives the Kind of `err`. An error that is not an `*Error` is `Internal`. It went across a boundary without a wrap, and that is a bug. |
| `(*Error).Error()` | Gives `Op: Message: Inner`. If the text of `Inner` is the same as `Message`, it gives only `Op: Message`. |

To get the `*Error` from an error, use `errors.AsType[*errs.Error](err)`.

### Boundaries in this repo

A boundary is a point where an error goes from one package to a different package. It
is also a point where an error goes from one goroutine to a different goroutine, or out
of the process.

```
 HTTP client / mdo CLI
        ▲  status code + Message + log ID           (4) top: log one time, show Message
 ┌──────┴────────────── cmd/server ───────────────┐
 │ handler: decode JSON, call Store, writeError   │  (3) wrap the handler's own errors
 └──────▲─────────────────────────────────────────┘
        │ *Error
 ┌──────┴────────────── internal/store ───────────┐
 │ SQLite.Query and SQLite.Create: exported API   │  (2) wrap foreign errors here
 │   registry.do ◄─ result chan ─ object goroutine│  (1) goroutine to owner: return it
 │     database/sql, mattn/go-sqlite3, os         │  foreign errors start here
 └────────────────────────────────────────────────┘
```

### When to wrap an error and when to send it on

| Location | Action |
|---|---|
| **In one package.** An unexported function calls a different unexported function. | Send the error on with no change. Or add context with `fmt.Errorf("open %s: %w", path, err)`. Do not make an `*Error` here. |
| **At the exit of a package.** An exported function returns to a different package. | Wrap each error that is not an `*Error` from this package: `errs.Wrap(err, errs.BadQuery, "store.Query", "bad query: "+err.Error())`. Select the Kind here. This package knows what the failure means. |
| **An `*Error` from a package that you called.** | If its Kind and Message are correct for your caller, send it on with no change. Wrap it again only to change the Kind or the Message. `Wrap` keeps the first stack. |
| **A goroutine that sends a result back.** An example is the object goroutine that sends to `registry.do`. | Return the error to the goroutine that owns the request, in the result value or the result channel. Do not write it to the log and then discard it. The owner decides what to do. |
| **A panic in a goroutine that serves many requests.** An example is the `run` loop of an object. | Recover at the top of that goroutine. Make the panic into an `Internal` `*Error` that has the stack. Return it to the request. If you do not recover, one bad request stops the process. |
| **`ctx.Err()`** | In the package, send it on with no change. At the boundary, wrap it as `Canceled`. In the Message, tell the caller if the deadline passed or if the caller stopped the request. |
| **The top.** This is the HTTP handler or the CLI `main`. | Write the error to the log one time. Include the log ID, Op, Inner, and Misc. For `Internal`, also include the Stack. Show the client only the `Message` and the log ID. An error that is not an `*Error` is a bug. Write all of it to the log, and show a general message. |

These rules come from the table:

- **Write an error to the log only at the top.** Do not write an error to the log and
  also return it. If you do, the log shows one failure many times.
- **The client sees only the `Message`.** `Inner` can contain file paths, driver text,
  or other internal details.
- **One exception applies to `BadQuery`.** The SQLite error is the useful message,
  because the SQL came from the client. Thus, the store copies the SQLite error into
  `Message`.
- **Record one stack for each `Internal` error, at the first wrap.** Later wraps add an
  Op and a Message. They do not add a stack. Client errors do not get a stack.
- **Compare errors by Kind,** for example `errs.KindOf(err) == errs.NotFound`. To find a
  specific cause, use `errors.Is` or `errors.As`. Do not compare error strings.

### The top level

The HTTP top level is `writeError` in `cmd/server/handler.go`. It does these steps:

1. It makes a log ID with `rand.Text()` from `crypto/rand`.
2. If the error is not an `*errs.Error`, it writes one log record at level Error, with
   the message `unwrapped error reached the top`. It sends 500
   `internal error (log id id)`.
3. If the error is an `*errs.Error`, it writes one log record with the message
   `request failed`. The level and the keys are in [Logs](#logs).
4. It sends `Message (log id id)` with the status for the Kind.

| Kind | HTTP status |
|---|---|
| `Invalid`, `BadQuery` | 400 |
| `NotFound` | 404 |
| `Canceled` | 503 |
| `Internal` | 500 |

The CLI top level is `main` and `userMessage` in `cmd/mdo/main.go`:

- An HTTP error response already contains the Message and the log ID from the server.
  The CLI shows it as `HTTP 400: bad query: not authorized (log id 3fa9c1d2e07b4a55)`.
- The CLI wraps its own errors with the Op `mdo <command>`. An example is a network
  error. For an `Internal` error, the CLI also shows the cause, because the operator
  needs it: `cannot reach the server: dial tcp ...: connection refused`.
- The CLI writes the text to stderr and stops with exit status 1.

### Tests

- Compare the Kind with `errs.KindOf(err)`. Do not compare the message text. One
  exception applies: if the message is part of the API contract, compare it. An example
  is the `BadQuery` text that the client reads.
- Write a test that sends a foreign error across the store boundary. The error comes
  from the driver or from `os`. Make sure that it comes out as an `*Error` with a stack.
  `TestSQLiteForeignErrorIsWrapped` in `internal/store/sqlite_test.go` is an example.
- When you change a boundary, change the related mutants in
  `agent_work/validation/mutate/main.go`. Then run the mutation script.

### Where the code is

| Rule | Code |
|---|---|
| The error shape | `internal/errs/errs.go` |
| Wrap at the store boundary | `wrapErr` in `internal/store/store.go`. `SQLite.Create` and `SQLite.Query` call it. In the package, unexported marker errors (`errInvalidID`, `errNotFound`, `errBadQuery`) go up with `%w`. |
| Recover a panic in the object goroutine | `call` in `internal/store/registry.go`. It makes the panic into an `Internal` error that has the panic stack. The object goroutine continues to serve requests. |
| Wrap the errors of the handler | `sql` in `cmd/server/handler.go`: bad JSON and bad numbers become `Invalid`. |
| The top level | `writeError` in `cmd/server/handler.go`, and `userMessage` in `cmd/mdo/main.go`. |
| The logger and its flags | `newLogger` in `cmd/server/main.go`. |
| The `request failed` record and its level | `writeError` in `cmd/server/handler.go`. One switch on the Kind picks the HTTP status and the log level. |
| The Debug `request` record | `logRequests` in `cmd/server/handler.go`. |

The store gives each Kind this Message:

| Cause | Kind | Message |
|---|---|---|
| The object ID is not valid | `Invalid` | `invalid object ID` |
| The object does not exist | `NotFound` | `object not found` |
| SQLite rejected the SQL, or the SQL failed | `BadQuery` | `bad query: ` and the SQLite text |
| The deadline passed | `Canceled` | `request timed out` |
| The caller stopped the request | `Canceled` | `request canceled` |
| All other causes | `Internal` | `storage error` |

## Logs

The server writes logs with `log/slog` from the standard library. This section tells
you which level to use, which keys to use, and which data to keep out of the log.

### The logger

- `main` in `cmd/server` makes one `*slog.Logger`. It gives that logger to the
  constructors `newRouter` and `store.NewSQLite`.
- Each package keeps the logger that it gets. Do not call the package-level functions,
  for example `slog.Info` or `log.Printf`. Then a test can give a logger that writes to
  a buffer, or `slog.New(slog.DiscardHandler)`.
- The CLI, `cmd/mdo`, does not write logs. It writes results to stdout and errors to
  stderr.

### Format and level

Two server flags set the logger. The server writes the log to stderr.

| Flag | Values | Default |
|---|---|---|
| `-log-format` | `text`: one `key=value` line for each record, for a person to read. `json`: one JSON object on each line (JSONL), for a tool to read. | `text` |
| `-log-level` | `debug`, `info`, `warn`, `error`. The logger writes the records at this level and at the levels above it. | `info` |

### Levels

| Level | Use it for | Examples |
|---|---|---|
| Error | A failure that an operator must examine. | An `Internal` error at the top. An unwrapped error at the top. |
| Warn | A problem that is not a bug. It can show that the server has too much work. | A `Canceled` error at the top: the request timed out, or the client stopped it. |
| Info | Usual events that are important. | The server started. An object started or stopped. A client error at the top: `Invalid`, `NotFound`, or `BadQuery`. |
| Debug | Detail about one request. Turn it on only to find a problem. | One record for each HTTP request. |

### Messages and keys

- The message is a short fixed phrase in lower case, for example `object activated`. Do
  not put data in the message. Put data in attributes.
- Use these keys. Use the same key for the same data each time.

| Key | Value |
|---|---|
| `log_id` | The log ID that the client sees. |
| `kind` | The error Kind: `internal`, `invalid`, `not_found`, `bad_query`, or `canceled`. |
| `op` | The operation. |
| `object` | The object ID. |
| `err` | The error text, from `Error()`. |
| Each key in `Misc` | The value from `Misc`. Each entry of `Misc` is a separate key, for example `object`. |
| `stack` | The stack. Only `Internal` errors have one. |
| `method`, `path`, `status`, `duration` | The HTTP request, in the Debug record. |
| `addr`, `data`, `idle` | The server settings, in the start record. |

In the `json` format, a duration (`duration`, `idle`) is a number of nanoseconds. In
the `text` format, it is a text such as `1m0s`.

These are the log records in this repo:

| Message | Level | Keys |
|---|---|---|
| `server started` | Info | `addr`, `data`, `idle` |
| `server stopped` | Error | `err` |
| `object activated` | Info | `object` |
| `object evicted` | Info | `object` |
| `request failed` | From the Kind | `log_id`, `kind`, `op`, `err`, the keys in `Misc`, `stack` |
| `unwrapped error reached the top` | Error | `log_id`, `err` |
| `request` | Debug | `method`, `path`, `status`, `duration` |

### Data to keep out of the log

- Do not write SQL text, SQL parameters, or request bodies to the log. They can
  contain user data.
- An error text can contain a part of the SQL, because SQLite puts it in the error. This
  is permitted.
