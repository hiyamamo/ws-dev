package procman

import (
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// outputFilter selects which process's output is mirrored to the terminal.
// idx == 0 shows every process; idx in [1, len(names)] shows only
// names[idx-1]. Log files always receive every line regardless of the filter.
type outputFilter struct {
	mu    sync.RWMutex
	names []string
	idx   int
}

// shows reports whether the given process's output should reach the terminal.
func (f *outputFilter) shows(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.idx == 0 || (f.idx <= len(f.names) && f.names[f.idx-1] == name)
}

// cycle advances to the next state and returns its label ("all" or a name).
func (f *outputFilter) cycle() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idx = (f.idx + 1) % (len(f.names) + 1)
	if f.idx == 0 {
		return "all"
	}
	return f.names[f.idx-1]
}

// setupInteractive enables Tab-to-switch output filtering when the server runs
// on an interactive terminal. Pressing Tab cycles which process's output is
// shown (all -> first -> ... -> last -> all). It returns a restore func that
// puts the terminal back to its previous mode, or nil when no interactive
// terminal is attached (e.g. piped output or fewer than two processes).
func setupInteractive(filter *outputFilter, status io.Writer) func() {
	if len(filter.names) < 2 {
		return nil
	}
	inFd := int(os.Stdin.Fd())
	if !isTerminal(inFd) || !isTerminal(int(os.Stdout.Fd())) {
		return nil
	}
	old, err := enterCbreak(inFd)
	if err != nil {
		return nil
	}

	_, _ = fmt.Fprintln(status, "[ws-dev] press Tab to switch visible output (now: all)")

	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n == 1 && buf[0] == '\t' {
				_, _ = fmt.Fprintf(status, "[ws-dev] showing: %s\n", filter.cycle())
			}
		}
	}()

	return func() { restoreTermios(inFd, old) }
}

// isTerminal reports whether fd refers to a terminal.
func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	return err == nil
}

// enterCbreak switches the terminal to cbreak mode: keystrokes are delivered
// one at a time without waiting for Enter and without echo, while output
// post-processing (\n -> \r\n) and signal generation (Ctrl+C) stay intact.
func enterCbreak(fd int) (*unix.Termios, error) {
	t, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return nil, err
	}
	old := *t
	t.Lflag &^= unix.ICANON | unix.ECHO
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, ioctlSetTermios, t); err != nil {
		return nil, err
	}
	return &old, nil
}

func restoreTermios(fd int, old *unix.Termios) {
	if old != nil {
		_ = unix.IoctlSetTermios(fd, ioctlSetTermios, old)
	}
}
