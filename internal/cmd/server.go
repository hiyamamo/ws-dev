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
		freshLogs  bool
	)
	c := &cobra.Command{
		Use:   "server [<worktree>]",
		Short: "Start configured processes in a worktree (stops any prior server first; defaults to the repository root when the worktree is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runServer(firstArg(args), portBase, logDir, background, freshLogs)
		},
		ValidArgsFunction: completeWorktrees,
	}
	c.Flags().IntVar(&portBase, "port-base", 0, "Base port exposed as {{.PortBase}} / WS_DEV_PORT_BASE (default: 3000 or $WS_DEV_PORT_BASE)")
	c.Flags().StringVar(&logDir, "log-dir", "", "Log directory relative to the worktree (overrides config and $WS_DEV_LOG_DIR)")
	c.Flags().BoolVarP(&background, "background", "b", false, "Start the server detached in the background and return immediately (stop it with 'ws-dev server stop')")
	c.Flags().BoolVar(&freshLogs, "fresh-logs", false, "Truncate the previous run's *.log files before starting (also enabled by fresh_logs: true in config)")
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

func runServer(worktreeArg string, portBase int, logDirFlag string, background, freshLogs bool) error {
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
		return startBackground(worktreeArg, portBase, logDirFlag, logAbs, freshLogs)
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

	if err := os.WriteFile(pidPath, []byte(pidFileContents(os.Getpid())), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(worktreePath, []byte(worktree), 0o644); err != nil {
		return err
	}
	defer func() { _ = os.Remove(pidPath) }()

	portBase = resolvePortBase(portBase)

	logAbs := filepath.Join(dir, resolveLogDir(rc.Config, logDirFlag))

	// Start from empty logs when asked (flag or config). The prior server is
	// already stopped, so nothing is writing to them; in background mode this
	// runs in the detached child, whose own server.log is opened with O_APPEND
	// and therefore safe to truncate under itself.
	if freshLogs || rc.Config.FreshLogs {
		if err := truncateLogs(logAbs); err != nil {
			return err
		}
	}

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

// truncateLogs empties every *.log in dir. A missing dir means there is
// nothing to clean.
func truncateLogs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		if err := os.Truncate(filepath.Join(dir, e.Name()), 0); err != nil {
			return fmt.Errorf("truncate %s: %w", e.Name(), err)
		}
	}
	return nil
}

// startBackground re-execs `ws-dev server` (without --background) as a detached
// process in its own session, redirecting its combined output to server.log in
// the log directory. The child runs the foreground flow, so it stops any prior
// server and records its own pid; `ws-dev server stop` stops it like any other.
func startBackground(worktreeArg string, portBase int, logDirFlag, logAbs string, freshLogs bool) error {
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
	if freshLogs {
		args = append(args, "--fresh-logs")
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

// pidFileContents renders the pid file: the pid on the first line and the
// process start time on the second, so a later reader can tell a recycled pid
// (e.g. after a reboot) apart from the recorded server.
func pidFileContents(pid int) string {
	s := strconv.Itoa(pid)
	if st := processStartTime(pid); st != "" {
		s += "\n" + st
	}
	return s + "\n"
}

// processStartTime returns the start time of pid as reported by
// `ps -o lstart=`, or "" when it cannot be determined. The value is only ever
// compared for equality, so its exact format does not matter.
func processStartTime(pid int) string {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// readServerPid reads pidPath and reports the recorded pid and whether that
// process is currently alive (checked with signal 0) and still the process the
// file was written for (start time comparison, when recorded). A missing,
// malformed, or stale pid file yields alive=false; stale files are removed on
// the way.
func readServerPid(pidPath string) (pid int, alive bool, err error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	pid, err = strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || pid <= 0 {
		_ = os.Remove(pidPath)
		return 0, false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(pidPath)
		return 0, false, nil
	}
	// Signal 0: check liveness.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(pidPath)
		return 0, false, nil
	}
	// Identity check: a recycled pid belongs to a process with a different
	// start time than the one recorded at write time. Files without a recorded
	// start time (written by older versions) skip this and rely on liveness.
	if len(lines) == 2 {
		recorded := strings.TrimSpace(lines[1])
		if now := processStartTime(pid); recorded != "" && now != "" && now != recorded {
			_ = os.Remove(pidPath)
			return 0, false, nil
		}
	}
	return pid, true, nil
}

// stopPrior reads pidPath, sends SIGTERM to the process, waits for it to exit,
// and returns whether a running process was stopped. Missing pid file is OK.
func stopPrior(pidPath string) (bool, error) {
	pid, alive, err := readServerPid(pidPath)
	if err != nil || !alive {
		return false, err
	}
	// FindProcess never fails on unix; readServerPid just confirmed liveness.
	proc, _ := os.FindProcess(pid)
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
