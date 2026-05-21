package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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
)

func newServerCmd() *cobra.Command {
	var (
		portBase int
		logDir   string
	)
	c := &cobra.Command{
		Use:   "server [<worktree>]",
		Short: "Start configured processes in a worktree (stops any prior server first; defaults to the repository root when the worktree is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runServer(firstArg(args), portBase, logDir)
		},
	}
	c.Flags().IntVar(&portBase, "port-base", 0, "Base port exposed as {{.PortBase}} / WS_DEV_PORT_BASE (default: 3000 or $WS_DEV_PORT_BASE)")
	c.Flags().StringVar(&logDir, "log-dir", "", "Log directory relative to the worktree (overrides config and $WS_DEV_LOG_DIR)")
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

func runServer(worktreeArg string, portBase int, logDirFlag string) error {
	rc, err := loadRepoCtx()
	if err != nil {
		return err
	}
	worktree, dir, err := rc.resolveWorktree(worktreeArg)
	if err != nil {
		return err
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

	return procman.Run(procman.Opts{
		Cfg:      rc.Config,
		Worktree: worktree,
		Dir:      dir,
		Root:     rc.Root,
		LogDir:   logAbs,
		PortBase: portBase,
	})
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
