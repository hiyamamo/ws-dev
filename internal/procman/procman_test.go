package procman

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
)

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
	outCh := make(chan outLine)
	printDone := make(chan struct{})
	go func() {
		defer close(printDone)
		for msg := range outCh {
			_, _ = msg.w.Write(msg.data)
		}
	}()

	filter := &outputFilter{names: names} // idx 0 => show all

	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			src := strings.NewReader(manyLines(name, linesPerProc))
			prefix := name + " | "
			copyTee(src, io.Discard, &buf, prefix, name, filter, outCh)
		}(name)
	}
	wg.Wait()    // all senders done
	close(outCh) // the one safe place to close
	<-printDone  // drain before reading buf

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
