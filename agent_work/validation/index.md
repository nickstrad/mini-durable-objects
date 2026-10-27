# Validation scripts

Scripts that check whether the `internal/store` tests catch real bugs, rather
than passing because they mirror the code. Results of past runs are in
[results.md](results.md).

Every script works on a temp copy of the repo and never modifies the working
tree. Go scripts are ordinary `package main` directories in the module, so
`make check` builds and vets them; they share `vlib/`. Run them from the repo
root. Mutants match on code only, never on comments, so editing a comment
doesn't break them.

## Scripts

| Script | What it does | Command |
|---|---|---|
| `mutate/` | Applies one small bug at a time to `registry.go`, `store.go`, or `sqlite.go`, runs `go test ./internal/store/`, and prints a killed/survived table. Exits 1 if a mutant survives, 2 if a mutant no longer matches the code. | `go run ./agent_work/validation/mutate -fuzztime 15s` (see `-h` for `-run`, `-only`, `-race`) |
| `stats/` | Instruments `TestRegistryRandomWorkloads` and prints how often each path is hit across its seeds, to show the property test isn't vacuous. | `go run ./agent_work/validation/stats` |
| `flaky.sh` | Runs the registry tests N times under the race detector. | `./agent_work/validation/flaky.sh 20` |
| `vlib/` | Shared helpers: `CopyRepo`, `Replace`, `GoTest`, `Summarize`. Not run directly. | - |
