package procman

import (
	"bufio"
	"bytes"
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
	"text/template"
	"time"

	"github.com/hiyamamo/ws-dev/internal/config"
	"github.com/hiyamamo/ws-dev/internal/tasks"
)

// Vars are template variables exposed to process cmd strings.
type Vars struct {
	Label     string
	PortBase  int
	Workspace string
}

type Opts struct {
	Cfg       *config.Config
	Label     string
	RepoDir   string
	LogDir    string // absolute path
	PortBase  int
	Workspace string // absolute path to workspace root
	Stdout    io.Writer
	Stderr    io.Writer
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

	var wg sync.WaitGroup
	var mu sync.Mutex
	children := map[string]*exec.Cmd{}

	for _, name := range names {
		p := o.Cfg.Processes[name]
		argv, err := buildArgv(o.Cfg, p.Cmd, Vars{Label: o.Label, PortBase: o.PortBase, Workspace: o.Workspace})
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
		cmd.Dir = o.RepoDir
		cmd.Env = append(os.Environ(),
			"WS_DEV_LABEL="+o.Label,
			"WS_DEV_WORKSPACE="+o.Workspace,
			fmt.Sprintf("WS_DEV_PORT_BASE=%d", o.PortBase),
			"WS_DEV_LOG_DIR="+o.LogDir,
		)
		for k, v := range p.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		prefix := padRight(name, maxNameLen) + " | "
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
		_, _ = fmt.Fprintf(o.Stdout, "[ws-dev] started %s (pid %d): %s\n", name, cmd.Process.Pid, strings.Join(argv, " "))

		wg.Add(2)
		go func() {
			defer wg.Done()
			copyTee(outPipe, logFile, o.Stdout, prefix)
		}()
		go func() {
			defer wg.Done()
			copyTee(errPipe, logFile, o.Stderr, prefix)
		}()
		wg.Add(1)
		go func(name string, c *exec.Cmd, lf *os.File) {
			defer wg.Done()
			err := c.Wait()
			_ = lf.Close()
			mu.Lock()
			delete(children, name)
			mu.Unlock()
			if err != nil {
				_, _ = fmt.Fprintf(o.Stdout, "[ws-dev] %s exited: %v\n", name, err)
			} else {
				_, _ = fmt.Fprintf(o.Stdout, "[ws-dev] %s exited cleanly\n", name)
			}
		}(name, cmd, logFile)
	}

	// Wait for signal or for all children to exit.
	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	select {
	case sig := <-sigCh:
		_, _ = fmt.Fprintf(o.Stdout, "[ws-dev] received %s, shutting down\n", sig)
	case <-allDone:
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
		_, _ = fmt.Fprintln(o.Stdout, "[ws-dev] timeout, sending SIGKILL")
		for _, c := range snapshot {
			if c.Process != nil {
				_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
			}
		}
		<-allDone
	}
	return nil
}

func buildArgv(cfg *config.Config, cmd string, v Vars) ([]string, error) {
	tmpl, err := template.New("cmd").Parse(cmd)
	if err != nil {
		return nil, fmt.Errorf("parse cmd template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return nil, fmt.Errorf("expand cmd template: %w", err)
	}
	return tasks.BuildArgv(cfg, buf.String(), nil), nil
}

func copyTee(src io.Reader, logFile io.Writer, stdout io.Writer, prefix string) {
	r := bufio.NewReader(src)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = logFile.Write(line)
			_, _ = stdout.Write([]byte(prefix))
			_, _ = stdout.Write(line)
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
