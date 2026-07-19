package procman

import (
	"bytes"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hiyamamo/ws-dev/internal/config"
)

// TestRunStartFailureStopsStartedProcesses verifies the startup-error path
// goes through the same group-kill shutdown as a signal: when a later process
// fails to start, the already-started ones are terminated (not waited for to
// completion) and Run returns the start error.
func TestRunStartFailureStopsStartedProcesses(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	o := Opts{
		Cfg: &config.RepoConfig{Processes: map[string]config.Process{
			// Sorted start order: the sleeper starts first, then the broken
			// process fails.
			"a-sleeper": {Cmd: "sleep 30"},
			"b-broken":  {Cmd: "/nonexistent/ws-dev-test-binary"},
		}},
		Worktree: "wt",
		Dir:      dir,
		Root:     dir,
		LogDir:   filepath.Join(dir, "log"),
		Stdout:   &out,
		Stderr:   &errOut,
	}
	start := time.Now()
	err := Run(o)
	if err == nil {
		t.Fatal("expected a start error")
	}
	if !strings.Contains(err.Error(), "b-broken") {
		t.Errorf("error should name the failing process, got: %v", err)
	}
	// Well under sleep's 30s and killAll's 5s SIGKILL escalation: the sleeper
	// must have been SIGTERMed, and Run must have waited for its goroutines.
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("Run took %v; started processes were not stopped", d)
	}
	if !strings.Contains(out.String(), "started a-sleeper") {
		t.Errorf("missing started message, out=%q", out.String())
	}
	if !strings.Contains(out.String(), "a-sleeper exited") {
		t.Errorf("sleeper exit was not waited for, out=%q", out.String())
	}
}

// manyLines builds "<name>-0\n<name>-1\n..." with n lines, used to generate
// enough concurrent output to surface interleaving without a real process.
func manyLines(name string, n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(name)
		b.WriteByte('-')
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\n')
	}
	return b.String()
}

// TestCopyTeePrinterKeepsLinesIntact runs several copyTee goroutines
// concurrently through the single printer goroutine and asserts that every
// console line is emitted as one unit: each line starts with a valid prefix
// and no line is split by another process's output. Run with -race to also
// catch concurrent writes to the shared console writer.
func TestCopyTeePrinterKeepsLinesIntact(t *testing.T) {
	const linesPerProc = 500
	names := []string{"web", "api"}

	var buf bytes.Buffer // only the printer writes here, so this is safe
	pr := newPrinter()

	filter := &outputFilter{names: names} // idx 0 => show all

	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			src := strings.NewReader(manyLines(name, linesPerProc))
			prefix := name + " | "
			copyTee(src, io.Discard, &buf, prefix, name, filter, pr)
		}(name)
	}
	wg.Wait()  // all senders done
	pr.close() // flush before reading buf

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if got, want := len(lines), linesPerProc*len(names); got != want {
		t.Fatalf("line count = %d, want %d (tail lines dropped?)", got, want)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "web | ") && !strings.HasPrefix(line, "api | ") {
			t.Fatalf("interleaved or split line: %q", line)
		}
	}
}
