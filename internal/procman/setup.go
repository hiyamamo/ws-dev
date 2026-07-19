package procman

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hiyamamo/ws-dev/internal/tasks"
)

// RunSetup executes the configured setup commands sequentially, before the
// server's processes start. Each entry is template-expanded with the same vars
// as process cmds ({{.Worktree}} etc.) and run via `sh -c` in the worktree
// directory with stdio inherited and the same WS_DEV_* environment processes
// receive.
//
// Setup commands are intentionally NOT wrapped by exec_wrapper: they are meant
// for bootstrap steps such as `direnv allow` / `mise trust` that must run as
// given (wrapping `direnv allow` in `direnv exec .` would be circular and fail
// before the .envrc is even authorized). A step that needs the toolchain can
// include the wrapper itself, e.g. `mise exec -- pnpm install`.
//
// The first command to exit non-zero aborts the sequence and is returned, so
// the server never starts on top of a failed setup. In background mode this
// runs in the detached child, so its output is captured in server.log.
func RunSetup(o Opts) error {
	if len(o.Cfg.Setup) == 0 {
		return nil
	}
	stdout := o.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := o.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	v := tasks.Vars{Worktree: o.Worktree, Root: o.Root, Dir: o.Dir, PortBase: o.PortBase}
	env := append(os.Environ(), tasks.Env(v, o.LogDir)...)

	for i, raw := range o.Cfg.Setup {
		cmdStr, err := tasks.Expand(raw, v)
		if err != nil {
			return fmt.Errorf("setup[%d]: %w", i, err)
		}
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}
		_, _ = fmt.Fprintf(stdout, "[ws-dev] setup: %s\n", cmdStr)

		c := exec.Command("sh", "-c", cmdStr)
		c.Dir = o.Dir
		c.Stdin = os.Stdin
		c.Stdout = stdout
		c.Stderr = stderr
		c.Env = env
		if err := c.Run(); err != nil {
			return fmt.Errorf("setup command failed (%s): %w", cmdStr, err)
		}
	}
	return nil
}
