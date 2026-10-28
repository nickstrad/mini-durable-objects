// Command mutate runs mutation testing against internal/store on a temp copy
// of the repo. Each mutant is a small set of old->new string replacements in
// non-test code. A mutant is "killed" when go test fails (an assertion, a
// panic, a synctest deadlock, or a timeout) and "survived" when it passes.
// A mutant whose old string is not found exactly once is an ERROR: the code
// under test has drifted and the mutant list needs updating. Exit status is
// 1 if any mutant survived and 2 if any mutant failed to apply.
//
//	go run ./agent_work/validation/mutate [-repo .] [-run REGEXP] [-only REGEXP] [-race] [-fuzztime 15s]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nickstrad/mini-durable-objects/agent_work/validation/vlib"
)

type mutant struct {
	Name  string
	File  string      // relative to internal/store
	Pairs [][2]string // match text is code only, never comments, so comment edits don't break mutants
}

const (
	evictCond  = "if obj.active == 0 && now.Sub(obj.lastUsed) >= r.idleTimeout {"
	openCall   = "\t\tres, err := r.open(path)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n"
	objectDef  = "type object[R io.Closer] struct {\n"
	newObject  = "\t\tobj = &object[R]{inbox: inbox, done: done, activatedAt: time.Now()}\n"
	closeDefer = "\tdefer res.Close()\n"
)

// withRes gives object a res field so a mutant can close it from the wrong
// place. These pairs are prepended to such mutants.
var withRes = [][2]string{
	{objectDef, objectDef + "\tres      R\n"},
	{newObject, "\t\tobj = &object[R]{inbox: inbox, done: done, activatedAt: time.Now(), res: res}\n"},
}

func withResField(pairs ...[2]string) [][2]string {
	return append(append([][2]string{}, withRes...), pairs...)
}

var mutants = []mutant{
	// run / inbox
	{"M01 run fn on its own goroutine", "registry.go", [][2]string{{"\t\t\tfn(res)\n", "\t\t\tgo fn(res)\n"}}},
	{"M02 buffered inbox (8)", "registry.go", [][2]string{{"inbox := make(chan func(R))", "inbox := make(chan func(R), 8)"}}},
	// active count / lastUsed
	{"M03 drop obj.active++", "registry.go", [][2]string{{"\tobj.active++\n", "\n"}}},
	{"M04 release does not decrement", "registry.go", [][2]string{{"\tobj.active--\n", "\n"}}},
	{"M05 release decrements twice", "registry.go", [][2]string{{"\tobj.active--\n", "\tobj.active -= 2\n"}}},
	{"M06 release forgets lastUsed", "registry.go", [][2]string{{"\tobj.lastUsed = time.Now()\n", "\n"}}},
	// evictIdle
	{"M07 evict ignores active", "registry.go", [][2]string{{evictCond, "if now.Sub(obj.lastUsed) >= r.idleTimeout {"}}},
	{"M08 evict ignores idle time", "registry.go", [][2]string{{evictCond, "if obj.active == 0 && now.Sub(obj.lastUsed) >= 0 {"}}},
	{"M09 evict uses > not >=", "registry.go", [][2]string{{"now.Sub(obj.lastUsed) >= r.idleTimeout", "now.Sub(obj.lastUsed) > r.idleTimeout"}}},
	{"M10 evict does not close done", "registry.go", [][2]string{{"\t\t\tclose(obj.done)\n", "\n"}}},
	{"M11 evict does not delete from map", "registry.go", [][2]string{{"\t\t\tdelete(r.objects, id)\n", "\n"}}},
	// do
	// A channel that is never ready: same as having no cancel case.
	{"M12 do has no ctx.Done case", "registry.go", [][2]string{{"case <-ctx.Done():", "case <-make(chan struct{}):"}}},
	{"M13 canceled do returns nil", "registry.go", [][2]string{{"\t\treturn ctx.Err()\n", "\t\treturn nil\n"}}},
	{"M14 do returns before result", "registry.go", [][2]string{{"\treturn <-result\n", "\treturn nil\n"}}},
	{"M15 do drops fn's error", "registry.go", [][2]string{{"func(res R) { result <- timedCall(t, sent, fn, res) }", "func(res R) { timedCall(t, sent, fn, res); result <- nil }"}}},
	{"M16 release before fn finishes", "registry.go", [][2]string{{"\tdefer r.release(obj, t)\n", "\tr.release(obj, t)\n"}}},
	// acquire
	{"M17 acquire never stores object (no warm reuse)", "registry.go", [][2]string{{"\t\tr.objects[id] = obj\n", "\n"}}},
	{"M18 acquire shares one object across ids", "registry.go", [][2]string{{
		"\tobj, ok := r.objects[id]\n",
		"\tobj, ok := r.objects[id]\n\tif !ok {\n\t\tfor _, o := range r.objects {\n\t\t\tobj, ok = o, true\n\t\t}\n\t}\n"}}},
	{"M19 active++ after the send", "registry.go", [][2]string{
		{"\tobj.active++\n", "\n"},
		{"\tcase obj.inbox <- func(res R) { result <- timedCall(t, sent, fn, res) }:\n",
			"\tcase obj.inbox <- func(res R) { result <- timedCall(t, sent, fn, res) }:\n\t\tr.mu.Lock()\n\t\tobj.active++\n\t\tr.mu.Unlock()\n"}}},
	// Returns before the inbox select, which becomes unreachable.
	{"M20 each request opens its own resource on its own goroutine", "registry.go", [][2]string{{"\tresult := make(chan error, 1)\n",
		"\tresult := make(chan error, 1)\n\tgo func() {\n\t\tres, err := r.open(path)\n\t\tif err != nil {\n\t\t\tresult <- err\n\t\t\treturn\n\t\t}\n\t\tdefer res.Close()\n\t\tresult <- fn(res)\n\t}()\n\treturn <-result\n"}}},
	{"M21 run ignores done", "registry.go", [][2]string{{"\t\tcase <-done:\n\t\t\treturn\n", "\t\tcase <-done:\n\t\t\tdone = nil\n"}}},
	{"M22 do drops the up-front ctx.Err check", "registry.go", [][2]string{{"\tif err := ctx.Err(); err != nil {\n\t\treturn err\n\t}\n", "\n"}}},
	// resource lifecycle (observable through the fake's close count)
	{"R01 run never closes res", "registry.go", [][2]string{{closeDefer, "\n"}}},
	{"R02 run closes res twice", "registry.go", [][2]string{{closeDefer, closeDefer + closeDefer}}},
	{"R03 run closes res before serving requests", "registry.go", [][2]string{{closeDefer, "\tres.Close()\n"}}},
	{"R04 run closes res after every request", "registry.go", [][2]string{{"\t\t\tfn(res)\n", "\t\t\tfn(res)\n\t\t\tres.Close()\n"}}},
	{"R05 acquire opens on every request (old goroutine orphaned)", "registry.go", [][2]string{{"\tobj, ok := r.objects[id]\n", "\tobj, ok := r.objects[id]\n\tok = false\n"}}},
	{"R06 acquire ignores open's error", "registry.go", [][2]string{{openCall, "\t\tres, _ := r.open(path)\n"}}},
	{"R07 acquire hands run the zero resource", "registry.go", [][2]string{{"\t\tgo run(done, inbox, res)\n", "\t\tvar zero R\n\t\tgo run(done, inbox, zero)\n\t\t_ = res\n"}}},
	{"R08 release closes res directly", "registry.go", withResField([2]string{"\tobj.lastUsed = time.Now()\n", "\tobj.lastUsed = time.Now()\n\tobj.res.Close()\n"})},
	{"R09 evict closes res directly instead of done", "registry.go", withResField([2]string{"\t\t\tclose(obj.done)\n", "\t\t\tobj.res.Close()\n"})},
	{"R10 evict closes res directly and done", "registry.go", withResField([2]string{"\t\t\tclose(obj.done)\n", "\t\t\tobj.res.Close()\n\t\t\tclose(obj.done)\n"})},
	// path / validID
	{"P01 validID allows '.'", "store.go", [][2]string{{"[a-z0-9-]{0,39}", "[a-z0-9.-]{0,39}"}}},
	{"P02 validID allows '/'", "store.go", [][2]string{{"[a-z0-9-]{0,39}", "[a-z0-9/-]{0,39}"}}},
	{"P03 validID allows uppercase after first char", "store.go", [][2]string{{"[a-z0-9-]{0,39}", "[a-zA-Z0-9-]{0,39}"}}},
	{"P04 validID allows length 41", "store.go", [][2]string{{"{0,39}", "{0,40}"}}},
	{"P05 validID allows leading '-'", "store.go", [][2]string{{"^[a-z0-9][a-z0-9-]{0,39}$", "^[a-z0-9-][a-z0-9-]{0,39}$"}}},
	{"P06 validID drops ^", "store.go", [][2]string{{"^[a-z0-9][a-z0-9-]{0,39}$", "[a-z0-9][a-z0-9-]{0,39}$"}}},
	{"P07 validID drops $", "store.go", [][2]string{{"^[a-z0-9][a-z0-9-]{0,39}$", "^[a-z0-9][a-z0-9-]{0,39}"}}},
	{"P08 validID allows '_'", "store.go", [][2]string{{"[a-z0-9-]{0,39}", "[a-z0-9_-]{0,39}"}}},
	{"P09 path joins id twice", "sqlite.go", [][2]string{{`filepath.Join(s.dir, id, "state.sqlite")`, `filepath.Join(s.dir, id, id, "state.sqlite")`}}},
	{"P10 path uses wrong filename", "sqlite.go", [][2]string{{`filepath.Join(s.dir, id, "state.sqlite")`, `filepath.Join(s.dir, id, "state.db")`}}},
	{"P11 path has no per-object dir", "sqlite.go", [][2]string{{`filepath.Join(s.dir, id, "state.sqlite")`, `filepath.Join(s.dir, id+".sqlite")`}}},
	{"P12 path returns a different error value", "sqlite.go", [][2]string{{"\t\treturn \"\", errInvalidID\n", "\t\treturn \"\", errors.New(\"invalid object ID\")\n"}}},
	{"P13 path skips validation", "sqlite.go", [][2]string{{"if !validID.MatchString(id) {", "if false && !validID.MatchString(id) {"}}},
	// panic recovery and the error boundary
	{"E01 call does not recover", "registry.go", [][2]string{{"\t\tif p := recover(); p != nil {", "\t\tif p := interface{}(nil); p != nil {"}}},
	{"E02 call recovers but drops the error", "registry.go", [][2]string{{"\t\t\terr = errs.Wrap(fmt.Errorf(\"panic: %v\", p), errs.Internal, \"store.registry\", \"storage error\")\n", "\t\t\t_ = errs.Wrap(fmt.Errorf(\"panic: %v\", p), errs.Internal, \"store.registry\", \"storage error\")\n"}}},
	{"E03 recovered panic has no stack", "registry.go", [][2]string{{"errs.Wrap(fmt.Errorf(\"panic: %v\", p), errs.Internal", "&errs.Error{Inner: fmt.Errorf(\"panic: %v\", p), Kind: errs.Internal"}, {"\"store.registry\", \"storage error\")", "Op: \"store.registry\"}"}}},
	{"E04 wrapErr treats deadline as canceled", "store.go", [][2]string{{"errors.Is(err, context.DeadlineExceeded)", "false"}}},
	{"E05 wrapErr drops the Canceled kind", "store.go", [][2]string{{"errs.Canceled, \"request canceled\"", "errs.Internal, \"request canceled\""}}},
	{"E06 wrapErr calls bad query internal", "store.go", [][2]string{{"errs.BadQuery, err.Error()", "errs.Internal, err.Error()"}}},
	{"E07 wrapErr drops not found", "store.go", [][2]string{{"errs.NotFound, errNotFound.Error()", "errs.Internal, errNotFound.Error()"}}},
	{"E08 Create skips the boundary wrap", "sqlite.go", [][2]string{{"err := wrapErr(\"store.Create\", id, s.create(ctx, id, &tm))", "err := s.create(ctx, id, &tm)"}}},
	{"E09 Query skips the boundary wrap", "query.go", [][2]string{{"\terr = wrapErr(\"store.Query\", id, err)\n", "\n"}}},
}

func main() {
	repo := flag.String("repo", ".", "path to the repo root (never modified)")
	run := flag.String("run", "", "go test -run filter, e.g. '^TestRegistryRandomWorkloads$' (default: whole package)")
	only := flag.String("only", "", "regexp selecting mutants by name")
	race := flag.Bool("race", false, "run go test with -race")
	fuzztime := flag.Duration("fuzztime", 0, "for surviving path mutants, retry with -fuzz FuzzObjectPath for this long")
	timeout := flag.Duration("timeout", 90*time.Second, "per-mutant go test timeout")
	flag.Parse()

	dir, cleanup, err := vlib.CopyRepo(*repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer cleanup()

	args := []string{"-count=1", "-timeout", timeout.String()}
	if *race {
		args = append(args, "-race")
	}
	if *run != "" {
		args = append(args, "-run", *run)
	}
	args = append(args, "./internal/store/")

	fmt.Printf("repo copy: %s\nbaseline: go test %s\n", dir, strings.Join(args, " "))
	if out, ok := vlib.GoTest(dir, *timeout+10*time.Second, args...); !ok {
		fmt.Println("BASELINE FAILED; not mutating:")
		fmt.Println(out)
		os.Exit(1)
	}
	fmt.Println("baseline: ok")

	var sel *regexp.Regexp
	if *only != "" {
		sel = regexp.MustCompile(*only)
	}
	killed, survived, errored := 0, 0, 0
	fmt.Println("\n| mutant | result | detail |\n|---|---|---|")
	for _, m := range mutants {
		if sel != nil && !sel.MatchString(m.Name) {
			continue
		}
		path := filepath.Join(dir, "internal", "store", m.File)
		orig, err := vlib.Replace(path, m.Pairs)
		if err != nil { // Replace writes nothing unless every pair matches.
			fmt.Printf("| %s | ERROR | %v |\n", m.Name, err)
			errored++
			continue
		}
		out, ok := vlib.GoTest(dir, *timeout+10*time.Second, args...)
		result, detail := "KILLED", strings.Join(vlib.Summarize(out, 3), " / ")
		// Path mutants (outside registry.go) get a fuzz retry if the seeds miss them.
		if ok && m.File != "registry.go" && *fuzztime > 0 {
			fout, fok := vlib.GoTest(dir, *fuzztime+60*time.Second,
				"-run", "^$", "-fuzz", "^FuzzObjectPath$", "-fuzztime", fuzztime.String(), "./internal/store/")
			os.RemoveAll(filepath.Join(dir, "internal", "store", "testdata", "fuzz"))
			if !fok {
				ok = false
				detail = "seed corpus passed; fuzz engine: " + strings.Join(vlib.Summarize(fout, 2), " / ")
			} else {
				detail = "survived seed corpus and " + fuzztime.String() + " of fuzzing"
			}
		}
		if ok {
			result = "SURVIVED"
			survived++
		} else {
			killed++
		}
		fmt.Printf("| %s | %s | %s |\n", m.Name, result, strings.ReplaceAll(detail, "|", "\\|"))
		if err := os.WriteFile(path, orig, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "restore failed:", err)
			os.Exit(2)
		}
	}
	fmt.Printf("\nkilled %d, survived %d, errored %d\n", killed, survived, errored)
	switch {
	case errored > 0:
		fmt.Println("ERROR: some mutants no longer match the code; update agent_work/validation/mutate/main.go")
		os.Exit(2)
	case survived > 0:
		os.Exit(1)
	}
}
