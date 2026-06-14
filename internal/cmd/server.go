package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/procman"
)

const (
	pidFileName      = "server.pid"
	worktreeFileName = "current-worktree"
	serverLogName    = "server.log"
)

func newServerCmd() *cobra.Command {
	var (
		portBase   int
		logDir     string
		background bool
	)
	c := &cobra.Command{
		Use:   "server [<worktree>]",
		Short: "Start configured processes in a worktree (stops any prior server first; defaults to the repository root when the worktree is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runServer(firstArg(args), portBase, logDir, background)
		},
	}
	c.Flags().IntVar(&portBase, "port-base", 0, "Base port exposed as {{.PortBase}} / WS_DEV_PORT_BASE (default: 3000 or $WS_DEV_PORT_BASE)")
	c.Flags().StringVar(&logDir, "log-dir", "", "Log directory relative to the worktree (overrides config and $WS_DEV_LOG_DIR)")
	c.Flags().BoolVarP(&background, "background", "b", false, "Start the server detached in the background and return immediately (stop it with 'ws-dev server stop')")
	c.AddCommand(newServerStopCmd())
	return c
}

func newServerStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the currently running ws-dev server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			sdir, err := stateDir()
			if err != nil {
				return err
			}
			stopped, err := stopPrior(filepath.Join(sdir, pidFileName))
			if err != nil {
				return err
			}
			if !stopped {
				fmt.Println("No ws-dev server running")
			} else {
				fmt.Println("Stopped")
			}
			return nil
		},
	}
}

func runServer(worktreeArg string, portBase int, logDirFlag string, background bool) error {
	rc, err := loadRepoCtx()
	if err != nil {
		return err
	}
	worktree, dir, err := rc.resolveWorktree(worktreeArg)
	if err != nil {
		return err
	}

	// Background mode: the parent only validates config + worktree (so errors
	// surface in the foreground) and then re-execs itself detached. The detached
	// child runs the normal foreground flow below, owning the pid file and
	// process lifecycle.
	if background {
		logAbs := filepath.Join(dir, resolveLogDir(rc.Config, logDirFlag))
		return startBackground(worktreeArg, portBase, logDirFlag, logAbs)
	}

	sdir, err := stateDir()
	if err != nil {
		return err
	}
	pidPath := filepath.Join(sdir, pidFileName)
	worktreePath := filepath.Join(sdir, worktreeFileName)

	if _, err := stopPrior(pidPath); err != nil {
		return err
	}

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(worktreePath, []byte(worktree), 0o644); err != nil {
		return err
	}
	defer func() { _ = os.Remove(pidPath) }()

	if portBase == 0 {
		if v := os.Getenv("WS_DEV_PORT_BASE"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				portBase = n
			}
		}
	}
	if portBase == 0 {
		portBase = 3000
	}

	logAbs := filepath.Join(dir, resolveLogDir(rc.Config, logDirFlag))

	opts := procman.Opts{
		Cfg:      rc.Config,
		Worktree: worktree,
		Dir:      dir,
		Root:     rc.Root,
		LogDir:   logAbs,
		PortBase: portBase,
	}

	// Run setup commands (direnv allow / mise trust / pnpm install, etc.) before
	// starting any process, so the server comes up in a prepared environment. A
	// failed setup aborts the start. In background mode this runs in the detached
	// child, so its output goes to server.log.
	if err := procman.RunSetup(opts); err != nil {
		return err
	}

	return procman.Run(opts)
}

// startBackground re-execs `ws-dev server` (without --background) as a detached
// process in its own session, redirecting its combined output to server.log in
// the log directory. The child runs the foreground flow, so it stops any prior
// server and records its own pid; `ws-dev server stop` stops it like any other.
func startBackground(worktreeArg string, portBase int, logDirFlag, logAbs string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if err := os.MkdirAll(logAbs, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	logPath := filepath.Join(logAbs, serverLogName)
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	// Pass the original flag values through unchanged so the child applies the
	// same env/default precedence (e.g. portBase 0 -> $WS_DEV_PORT_BASE -> 3000).
	args := []string{"server"}
	if worktreeArg != "" {
		args = append(args, worktreeArg)
	}
	if portBase != 0 {
		args = append(args, "--port-base", strconv.Itoa(portBase))
	}
	if logDirFlag != "" {
		args = append(args, "--log-dir", logDirFlag)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background server: %w", err)
	}
	fmt.Printf("[ws-dev] server started in background (pid %d)\n", cmd.Process.Pid)
	fmt.Printf("[ws-dev] logs: %s\n", logPath)
	return nil
}

// stopPrior reads pidPath, sends SIGTERM to the process, waits for it to exit,
// and returns whether a running process was stopped. Missing pid file is OK.
func stopPrior(pidPath string) (bool, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		_ = os.Remove(pidPath)
		return false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(pidPath)
		return false, nil
	}
	// Signal 0: check liveness.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(pidPath)
		return false, nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return false, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			_ = os.Remove(pidPath)
			return true, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = proc.Signal(syscall.SIGKILL)
	_ = os.Remove(pidPath)
	return true, nil
}
