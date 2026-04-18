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
	"github.com/hiyamamo/ws-dev/internal/workspace"
)

const (
	pidFileName   = "server.pid"
	labelFileName = "current-label"
)

func newServerCmd() *cobra.Command {
	var (
		portBase int
		logDir   string
	)
	c := &cobra.Command{
		Use:   "server <label>",
		Short: "Start configured processes for a label (stops any prior server first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runServer(args[0], portBase, logDir)
		},
	}
	c.Flags().IntVar(&portBase, "port-base", 0, "Base port exposed as {{.PortBase}} / WS_DEV_PORT_BASE (default: 3000 or $WS_DEV_PORT_BASE)")
	c.Flags().StringVar(&logDir, "log-dir", "", "Log directory relative to repo (overrides config and $WS_DEV_LOG_DIR)")
	c.AddCommand(newServerStopCmd())
	return c
}

func newServerStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the currently running ws-dev server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ws, err := workspace.FindFromCwd()
			if err != nil {
				return err
			}
			stateDir, err := ws.StateDir()
			if err != nil {
				return err
			}
			stopped, err := stopPrior(filepath.Join(stateDir, pidFileName))
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

func runServer(label string, portBase int, logDirFlag string) error {
	ws, err := workspace.FindFromCwd()
	if err != nil {
		return err
	}
	repoDir := ws.RepoDir(label)
	if _, err := os.Stat(repoDir); err != nil {
		return fmt.Errorf("repo dir %s not found (run `ws-dev clone %s` first)", repoDir, label)
	}

	stateDir, err := ws.StateDir()
	if err != nil {
		return err
	}
	pidPath := filepath.Join(stateDir, pidFileName)
	labelPath := filepath.Join(stateDir, labelFileName)

	if _, err := stopPrior(pidPath); err != nil {
		return err
	}

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(labelPath, []byte(label), 0o644); err != nil {
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

	logSub := ws.ResolveLogDir(logDirFlag)
	logAbs := filepath.Join(repoDir, logSub)

	return procman.Run(procman.Opts{
		Cfg:       ws.Config,
		Label:     label,
		RepoDir:   repoDir,
		LogDir:    logAbs,
		PortBase:  portBase,
		Workspace: ws.Root,
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
