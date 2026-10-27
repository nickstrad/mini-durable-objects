# Validation results

Results of runs of the scripts listed in [index.md](index.md). The
scripts print their own tables; this file keeps only what each run found.

## Run 2 (generic registry)

What changed since run 1:

- `registry[R io.Closer]` is generic with an injected `open func(path) (R, error)`;
  `run` does `defer res.Close()`. Tests use `fakeRes` (counts `Close` calls) via
  `newFakeRegistry`. All 34 run-1 mutants were retargeted to the new code (`*sql.DB`
  -> `R`, `openDB` -> `r.open`); a mutant that no longer matches is now a hard
  error (exit 2) instead of a table row that is easy to miss.
- Ten resource-lifecycle mutants added (R01-R10): run never closes / closes twice /
  closes before serving / closes after every request; acquire opens on every
  request / ignores open's error / hands run the zero resource; release or
  evictIdle close the resource directly (with and without closing `done`).
- Run 1's verified fixes were applied to the real tests: `canceledWhileBlocked`
  WaitGroup and the `idle == time.Hour` no-eviction check in
  `TestRegistryRandomWorkloads`; seed `"aB"` in `FuzzObjectPath`. The property
  test also gained a close-count check: every fn asserts its resource is not
  closed, and after `stopAll` every resource ever opened is closed exactly once.

Results. "Full" = whole package; "New" = the five new registry tests +
`FuzzObjectPath`; "Prop" = `TestRegistryRandomWorkloads` alone (run for the
resource and cancel mutants). A synctest deadlock aborts the package run, so the
killing test is the first one that failed.

| Mutant | Full | New | Killed by (new only) / note |
|---|---|---|---|
| M01 run fn on its own goroutine | killed | killed | RandomWorkloads: "2 fns running at once on a" |
| M02 buffered inbox (8) | killed | killed (Prop: killed) | **now** RandomWorkloads `canceledWhileBlocked` deadlock (survived in run 1) |
| M03 drop `obj.active++` | killed | killed | WaitingRequestKeepsObjectAlive (deadlock) |
| M04 release does not decrement | killed | killed | WaitingRequestKeepsObjectAlive (deadlock) |
| M05 release decrements twice | killed | killed | WaitingRequestKeepsObjectAlive (deadlock) |
| M06 release forgets lastUsed | killed | killed (Prop: killed) | **now** RandomWorkloads "b was evicted although idle for less than 1h" (survived in run 1) |
| M07 evict ignores active | killed | killed | WaitingRequest: "object with a waiting request was evicted" |
| M08 evict ignores idle time | killed | killed (Prop: killed) | **now** RandomWorkloads idle-1h check (survived in run 1) |
| M09 evict uses `>` not `>=` | killed | killed | synctest-frozen-clock artifact (see run 1) |
| M10 evict does not close done | killed | killed | goroutine leak caught by synctest.Test |
| M11 evict does not delete from map | killed | killed | ReactivatesWithFreshDB "idle object was not evicted" |
| M12 do has no `ctx.Done` case | killed | killed (Prop: killed) | **now** RandomWorkloads `canceledWhileBlocked` deadlock (survived in run 1) |
| M13 canceled do returns nil | killed | killed | AlreadyCanceledRequest |
| M14 do returns before result | killed | killed | WaitingRequest |
| M15 do drops fn's error | killed | killed | ReturnsFnError |
| M16 release before fn finishes | killed | killed | WaitingRequest |
| M17 acquire never stores object | killed | killed | WaitingRequest |
| M18 acquire shares one object across ids | killed | killed | WaitingRequest deadlock |
| M19 `active++` after the send | killed | killed | AlreadyCanceledRequest `active = -1` |
| M20 each request opens its own resource/goroutine | killed | killed (Prop: killed) | ReactivatesWithFreshDB "active resource closed 1 times"; Prop: exclusion |
| M21 run ignores done | killed | killed | goroutine leak |
| R01 run never closes res | killed | killed (Prop: killed) | ReactivatesWithFreshDB "closed 0 times, want 1"; Prop: close-count |
| R02 run closes res twice | killed | killed (Prop: killed) | "closed 2 times, want 1" |
| R03 run closes res before serving requests | killed | killed (Prop: killed) | Reactivates "active resource closed 1 times"; Prop: "got a closed resource" |
| R04 run closes res after every request | killed | killed (Prop: killed) | Reactivates; Prop: "got a closed resource" |
| R05 acquire opens on every request | killed | killed (Prop: killed) | WaitingRequest; Prop: exclusion violated (two goroutines per id) |
| R06 acquire ignores open's error | killed | killed | Survived at first; now killed by `TestRegistryReturnsOpenError` (see below). |
| R07 acquire hands run the zero resource | killed | killed (Prop: killed) | nil-pointer panic in `fakeRes.Close` at eviction |
| R08 release closes res directly | killed | killed (Prop: killed) | Reactivates "closed 2 times"; Prop: "got a closed resource" |
| R09 evict closes res directly instead of done | killed | killed (Prop: killed) | goroutine leak (done never closed) |
| R10 evict closes res directly and done | killed | killed (Prop: killed) | "closed 2 times, want 1" |
| P01-P13 (validID / path) | killed | killed | as in run 1; P03 is **now killed by the seed corpus** (`aB`) instead of needing the fuzz engine |

Totals: full suite 44/44 after adding `TestRegistryReturnsOpenError` (re-run
with `go run ./agent_work/validation/mutate`). Before that test: 43/44.

### R06: gap closed

`acquire` dropping `open`'s error was invisible because every test's `open`
succeeded. `TestRegistryReturnsOpenError` in `internal/store/registry_test.go`
now makes `open` fail and checks that `do` returns the error, fn never runs,
and no object is left registered.

### Test-quality notes (run 2)

- Stats (`go run ./agent_work/validation/stats`) are unchanged in shape: 6960
  requests; 1581 activations and 1581 evictions over 140 seeds (the 55
  `idle=1h` seeds have none); 505 requests canceled while genuinely waiting in a
  busy inbox; 4810 requests arrived at a busy object. The new assertions bite on
  those paths rather than on new schedules.
- The close-count check is behavioral (a resource is closed exactly once, never
  while in use) and uses only the fake's `Close` counter; it does not inspect
  `inbox`, `done`, or the select shape.
- R07's kill is a nil-pointer panic in the fake's `Close`, not an assertion; a
  real `*sql.DB` would also panic on `Close` of nil, so it is a fair kill, but
  the message is indirect.

### Flakiness (run 2)

`./agent_work/validation/flaky.sh 20`: `go test -race -count=20 -run '^TestRegistry' ./internal/store/`
passed (4.3s) after the test edits. The edited tests also passed `-race -count=3`
in the real repo, and `make check` passes.

## Run 1 (pre-generic registry, `*sql.DB` directly) — summary

34 mutants (M01-M21, P01-P13). Full suite 34/34 killed. New tests only 30/34:
survivors were M02 (buffered inbox), M12 (no `ctx.Done` case), M06 (lastUsed not
updated), M08 (idle time ignored). All four were caught only by the older
`CanceledRequestLeavesInbox` / `ReusesWarmObject`; the property test's cancel
checks were pure consistency checks (nil iff fn ran), and its `idle` parameter
observed nothing. P03 (uppercase after the first char) survived the seed corpus
and was found by the fuzz engine in under a second (`0A`). M09 (`>` vs `>=`) is
equivalent in real time; its kill comes from synctest's frozen clock making `>`
never evict, so `stopAll` leaks goroutines.

Run-1 flakiness: `-race -count=20` passed on two independent copies.

Run 1's fixes (verified then, applied in run 2): `canceledWhileBlocked`
WaitGroup waited on before `close(unblock)` (sound because the blocked object's
goroutine is held until `unblock`, so an already-canceled request can never be
accepted by an unbuffered inbox); `idle == time.Hour` => every object that ran a
request is still registered; seed `"aB"`.

## Run 3: canceled requests never run (2026-10-26)

`registry.do` now returns `ctx.Err()` before acquiring the object, so a request
already canceled when `do` is called never runs and never activates the object.
Before, an idle object's inbox and `ctx.Done()` were both ready and `select`
picked one at random, so such a request ran about half the time.

- `TestRegistryAlreadyCanceledRequest` now requires `context.Canceled`, fn not
  run, and no object activated. `TestRegistryRandomWorkloads` requires the same
  for requests canceled before `do`.
- New mutant **M22** (drop the up-front check) is killed by
  `TestRegistryAlreadyCanceledRequest`.
- M12 and M20 used the old select comment as their match text and errored
  loudly until updated, which is the runner working as intended.

Result: `go run ./agent_work/validation/mutate` → killed 45, survived 0, errored 0.

## Run 4: after the /simplify cleanup (2026-10-26)

Mutants and stats now match on code, not comments (M12, M20 and the stats
`evictIdle` anchor were re-anchored). Registry tests use plain path labels and
stop the registry in `t.Cleanup`. `mutate -fuzztime 15s`: killed 45, survived 0,
errored 0. `stats`: cancel mode 1 (canceled before `do`) is now nil=0, so
already-canceled requests never run. `flaky.sh 20`: pass.
