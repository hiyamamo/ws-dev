package procman

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hiyamamo/ws-dev/internal/config"
	"github.com/hiyamamo/ws-dev/internal/tasks"
)

type Opts struct {
	Cfg      *config.RepoConfig
	Worktree string // worktree name
	Dir      string // absolute worktree directory (process cwd)
	Root     string // absolute main worktree root
	LogDir   string // absolute path
	PortBase int
	Stdout   io.Writer
	Stderr   io.Writer
}

// outLine is one unit of console output handed to the single printer
// goroutine. data already contains prefix+line concatenated, so the printer
// writes it in one Write and lines from different processes never interleave.
type outLine struct {
	w    io.Writer // target console writer (o.Stdout or o.Stderr)
	data []byte    // prefix + line, ready to write as one unit
}

// printer serializes all console output — process lines and [ws-dev] status
// messages alike — through one goroutine, so concurrent writers never split
// each other's lines. Log files are not routed through it; only the console
// writers are shared.
type printer struct {
	ch     chan outLine
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

func newPrinter() *printer {
	p := &printer{ch: make(chan outLine), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		for msg := range p.ch {
			_, _ = msg.w.Write(msg.data)
		}
	}()
	return p
}

// write queues one pre-formatted chunk to be written to w as a unit. After
// close, late senders (e.g. the Tab-filter notifier goroutine, which outlives
// Run) fall back to writing directly instead of panicking on a closed channel.
func (p *printer) write(w io.Writer, data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		_, _ = w.Write(data)
		return
	}
	p.ch <- outLine{w: w, data: data}
}

// statusf queues a "[ws-dev] ..." status line.
func (p *printer) statusf(w io.Writer, format string, args ...any) {
	p.write(w, fmt.Appendf(nil, "[ws-dev] "+format+"\n", args...))
}

// close flushes everything queued and stops the printer goroutine. Callers
// must ensure no copyTee sender is active (they are, transitively, waited on
// before close is reached); the main goroutine is the only sender afterwards.
func (p *printer) close() {
	p.mu.Lock()
	p.closed = true
	close(p.ch)
	p.mu.Unlock()
	<-p.done
}

// Run starts all configured processes and blocks until a shutdown signal
// is received or all child processes exit.
func Run(o Opts) error {
	if len(o.Cfg.Processes) == 0 {
		return fmt.Errorf("no processes defined in config")
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if err := os.MkdirAll(o.LogDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Sort process names for stable start order.
	names := make([]string, 0, len(o.Cfg.Processes))
	for n := range o.Cfg.Processes {
		names = append(names, n)
	}
	sort.Strings(names)

	maxNameLen := 0
	for _, n := range names {
		if len(n) > maxNameLen {
			maxNameLen = len(n)
		}
	}

	pr := newPrinter()
	filter := &outputFilter{names: names}
	if restore := setupInteractive(filter, func(format string, args ...any) {
		pr.statusf(o.Stdout, format, args...)
	}); restore != nil {
		defer restore()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	children := map[string]*exec.Cmd{}

	for _, name := range names {
		p := o.Cfg.Processes[name]
		argv, err := buildArgv(o.Cfg, p.Cmd, tasks.Vars{Worktree: o.Worktree, Root: o.Root, Dir: o.Dir, PortBase: o.PortBase})
		if err != nil {
			cancel()
			return err
		}
		if len(argv) == 0 {
			continue
		}
		logPath := filepath.Join(o.LogDir, name+".log")
		logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if err != nil {
			cancel()
			return fmt.Errorf("open log %s: %w", logPath, err)
		}

		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = o.Dir
		cmd.Env = append(os.Environ(), tasks.Env(tasks.Vars{Worktree: o.Worktree, Root: o.Root, Dir: o.Dir, PortBase: o.PortBase}, o.LogDir)...)
		for k, v := range p.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		prefix := o.Worktree + ":" + padRight(name, maxNameLen) + " | "
		outPipe, err := cmd.StdoutPipe()
		if err != nil {
			_ = logFile.Close()
			cancel()
			return err
		}
		errPipe, err := cmd.StderrPipe()
		if err != nil {
			_ = logFile.Close()
			cancel()
			return err
		}
		if err := cmd.Start(); err != nil {
			_ = logFile.Close()
			cancel()
			return fmt.Errorf("start %s: %w", name, err)
		}

		mu.Lock()
		children[name] = cmd
		mu.Unlock()
		pr.statusf(o.Stdout, "started %s (pid %d): %s", name, cmd.Process.Pid, strings.Join(argv, " "))

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Drain BOTH pipes to EOF before calling Wait. os/exec closes the
			// pipe fds inside Wait, so reading after Wait races with that close
			// and can silently drop the final lines (see StdoutPipe docs).
			var ioWg sync.WaitGroup
			ioWg.Add(2)
			go func() {
				defer ioWg.Done()
				copyTee(outPipe, logFile, o.Stdout, prefix, name, filter, pr)
			}()
			go func() {
				defer ioWg.Done()
				copyTee(errPipe, logFile, o.Stderr, prefix, name, filter, pr)
			}()
			ioWg.Wait()

			err := cmd.Wait()
			_ = logFile.Close()
			mu.Lock()
			delete(children, name)
			mu.Unlock()
			if err != nil {
				pr.statusf(o.Stdout, "%s exited: %v", name, err)
			} else {
				pr.statusf(o.Stdout, "%s exited cleanly", name)
			}
		}()
	}

	// Wait for signal or for all children to exit. Every per-process goroutine
	// (and its copyTee senders) is done once allDone closes; only the main
	// goroutine sends to the printer afterwards, so it can safely close it.
	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	select {
	case sig := <-sigCh:
		pr.statusf(o.Stdout, "received %s, shutting down", sig)
	case <-allDone:
		pr.close() // flush everything queued before returning
		return nil
	}

	// Graceful shutdown: SIGTERM to each process group, then SIGKILL after timeout.
	mu.Lock()
	snapshot := make(map[string]*exec.Cmd, len(children))
	for k, v := range children {
		snapshot[k] = v
	}
	mu.Unlock()
	for _, c := range snapshot {
		if c.Process != nil {
			_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
		}
	}
	select {
	case <-allDone:
	case <-time.After(5 * time.Second):
		pr.statusf(o.Stdout, "timeout, sending SIGKILL")
		for _, c := range snapshot {
			if c.Process != nil {
				_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
			}
		}
		<-allDone
	}
	pr.close() // flush everything queued before returning
	return nil
}

func buildArgv(cfg *config.RepoConfig, cmd string, v tasks.Vars) ([]string, error) {
	expanded, err := tasks.Expand(cmd, v)
	if err != nil {
		return nil, err
	}
	return tasks.BuildArgv(cfg, expanded, nil), nil
}

func copyTee(src io.Reader, logFile io.Writer, w io.Writer, prefix, name string, filter *outputFilter, pr *printer) {
	r := bufio.NewReader(src)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = logFile.Write(line)
			if filter.shows(name) {
				// Concatenate prefix+line into a fresh buffer and hand it to the
				// printer as one unit. The copy is required: line is only valid
				// until the next ReadBytes, but the printer reads it later.
				buf := make([]byte, 0, len(prefix)+len(line))
				buf = append(buf, prefix...)
				buf = append(buf, line...)
				pr.write(w, buf)
			}
		}
		if err != nil {
			return
		}
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
