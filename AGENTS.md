# mini-durable-objects

A 10-lesson learning project: a small single-machine runtime inspired by Cloudflare Durable
Objects. Each lesson adds one concept (lesson 1: filesystem identity and object routing; SQLite
arrives in lesson 2). The code grows one lesson at a time, so the history stays readable as a
course.

```
HTTP Client / Coding Agent -> Object Router -> resolve object ID -> Object A / Object B
                                                                    -> SQLite A / SQLite B
```

Language: Go. This is a separate project from `../durable-actor`; do not share its code or
assume its conventions.

**Current position:** Lesson 1 in progress. Update this line as each lesson lands.

## Staying on pace with the course

This is a learning project, not production code. When simplifying, reviewing, or refactoring,
keep changes small and in scope.

- Work only on the current lesson's step. Do not build features from later lessons early,
  even when a review suggests them.
- Do not make aggressive changes. Avoid large rewrites, restructuring, renames, new
  abstractions, or "cleanups" the lesson did not ask for. Prefer the smallest edit that fixes
  a real problem.
- Keep earlier lessons' constructs unless the current lesson replaces them. Folding them
  together is a lesson decision, not a cleanup.
- Prefer the simplest code that teaches the lesson's concept. Avoid performance work and
  hardening unless it serves the lesson.
- Each lesson's change ships with tests or a runnable demo showing the concept working.
- If a suggested change looks like it belongs to a later lesson or is a large departure from
  the current code, say so and ask before doing it.

## Go conventions

- Run `make check` (gofmt, `go vet`, and `make test`) before finishing any change.
- Standard library only, unless a lesson calls for a dependency (e.g. SQLite in lesson 2).
