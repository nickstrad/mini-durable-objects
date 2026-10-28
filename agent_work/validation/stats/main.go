// Command stats instruments TestRegistryRandomWorkloads on a temp copy of the
// repo and reports how often each interesting path is hit across its 200
// seeds: activations and evictions, outcomes per cancel mode, requests that
// were canceled while waiting in a busy object's inbox, and requests that
// arrived while their object was busy. It answers "is the property test
// vacuous?" with numbers instead of a guess.
//
//	go run ./agent_work/validation/stats [-repo .]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nickstrad/mini-durable-objects/agent_work/validation/vlib"
)

var registryEdits = [][2]string{
	{"import (\n\t\"context\"", "import (\n\t\"context\"\n\t\"sync/atomic\""},
	{"\t\tr.logger.Info(\"object activated\", \"object\", id)\n", "\t\tr.logger.Info(\"object activated\", \"object\", id)\n\t\tstatActivate.Add(1)\n"},
	{"\t\t\tr.logger.Info(\"object evicted\", \"object\", id)\n", "\t\t\tr.logger.Info(\"object evicted\", \"object\", id)\n\t\t\tstatEvict.Add(1)\n"},
	{"func (r *registry[R]) evictIdle() {", "var statActivate, statEvict atomic.Int64\n\nfunc (r *registry[R]) evictIdle() {"},
}

var testEdits = [][2]string{
	{"func TestRegistryRandomWorkloads(t *testing.T) {\n",
		"var statMode [3][2]atomic.Int64 // [cancel mode][0 = returned nil, 1 = returned Canceled]\n" +
			"var statWaitCanceled, statBusyAtCall, statSeedsWithEvict, statRequests atomic.Int64\n" +
			"var statIdle [3]atomic.Int64\n\n" +
			"func TestRegistryRandomWorkloads(t *testing.T) {\n" +
			"\tdefer func() {\n" +
			"\t\tt.Logf(\"STATS requests=%d activations=%d evictions=%d seedsWithEviction=%d idle0/1ms/1h seeds=%d/%d/%d\",\n" +
			"\t\t\tstatRequests.Load(), statActivate.Load(), statEvict.Load(), statSeedsWithEvict.Load(), statIdle[0].Load(), statIdle[1].Load(), statIdle[2].Load())\n" +
			"\t\tfor m := range 3 {\n" +
			"\t\t\tt.Logf(\"STATS cancel mode %d: nil=%d canceled=%d\", m, statMode[m][0].Load(), statMode[m][1].Load())\n" +
			"\t\t}\n" +
			"\t\tt.Logf(\"STATS canceledWhileWaitingInInbox=%d busyAtCall=%d\", statWaitCanceled.Load(), statBusyAtCall.Load())\n" +
			"\t}()\n"},
	{"\tidle := []time.Duration{0, time.Millisecond, time.Hour}[rng.IntN(3)]\n",
		"\tidleIdx := rng.IntN(3)\n\tstatIdle[idleIdx].Add(1)\n\tidle := []time.Duration{0, time.Millisecond, time.Hour}[idleIdx]\n\tevictBefore := statEvict.Load()\n"},
	{"\t\t\ttime.Sleep(start)\n\t\t\treq.err = r.do(",
		"\t\t\ttime.Sleep(start)\n" +
			"\t\t\tstatRequests.Add(1)\n" +
			"\t\t\tpre := ctx.Err() != nil\n" +
			"\t\t\tif inFlight[req.id].Load() > 0 {\n\t\t\t\tstatBusyAtCall.Add(1)\n\t\t\t}\n" +
			"\t\t\tdefer func() {\n" +
			"\t\t\t\tif req.err == nil {\n\t\t\t\t\tstatMode[mode][0].Add(1)\n\t\t\t\t} else {\n\t\t\t\t\tstatMode[mode][1].Add(1)\n\t\t\t\t\tif !pre {\n\t\t\t\t\t\tstatWaitCanceled.Add(1)\n\t\t\t\t\t}\n\t\t\t\t}\n" +
			"\t\t\t}()\n" +
			"\t\t\treq.err = r.do("},
	{"\tr.mu.Unlock()\n\n\tstopAll(r)\n", "\tr.mu.Unlock()\n\tif statEvict.Load() > evictBefore {\n\t\tstatSeedsWithEvict.Add(1)\n\t}\n\n\tstopAll(r)\n"},
}

func main() {
	repo := flag.String("repo", ".", "path to the repo root (never modified)")
	flag.Parse()

	dir, cleanup, err := vlib.CopyRepo(*repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer cleanup()

	store := filepath.Join(dir, "internal", "store")
	if _, err := vlib.Replace(filepath.Join(store, "registry.go"), registryEdits); err != nil {
		fmt.Fprintln(os.Stderr, "instrumenting registry.go:", err)
		os.Exit(2)
	}
	if _, err := vlib.Replace(filepath.Join(store, "registry_test.go"), testEdits); err != nil {
		fmt.Fprintln(os.Stderr, "instrumenting registry_test.go:", err)
		os.Exit(2)
	}
	out, ok := vlib.GoTest(dir, 2*time.Minute, "-count=1", "-v", "-run", "^TestRegistryRandomWorkloads$", "./internal/store/")
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "STATS") || strings.HasPrefix(l, "ok") || strings.HasPrefix(l, "FAIL") || strings.HasPrefix(l, "--- ") {
			fmt.Println(strings.TrimSpace(l))
		}
	}
	if !ok {
		fmt.Println(out)
		os.Exit(1)
	}
}
