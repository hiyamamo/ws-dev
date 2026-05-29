package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/git"
	"github.com/hiyamamo/ws-dev/internal/mcp"
)

func newMcpCmd() *cobra.Command {
	var logDirFlag string
	c := &cobra.Command{
		Use:   "mcp",
		Short: "Run the stdio MCP server (log operations for the current directory)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			// Anchor the log directory to the worktree that contains cwd so
			// the MCP server only ever sees its own worktree's logs. Fall back
			// to cwd when not inside a git worktree (mcp is intentionally
			// usable anywhere, even before the repo is registered).
			base := cwd
			if top, terr := git.Toplevel(); terr == nil {
				base = top
			}
			logDir := resolveMcpLogDir(base, logDirFlag, os.Getenv("WS_DEV_LOG_DIR"))
			return mcp.NewServer(logDir).Run()
		},
	}
	c.Flags().StringVar(&logDirFlag, "log-dir", "", "Log directory (overrides $WS_DEV_LOG_DIR)")
	return c
}

// resolveMcpLogDir resolves the MCP server's log directory, anchored to the
// worktree root base. Precedence: --log-dir flag > $WS_DEV_LOG_DIR > "log".
//
// A relative value is joined onto base. An absolute --log-dir flag is honored
// as-is (explicit operator intent). An absolute $WS_DEV_LOG_DIR is only honored
// when it points inside base: `ws-dev server` (procman) exports WS_DEV_LOG_DIR
// as an absolute path for the worktree it runs in, and that value is inherited
// by any MCP server launched in the same environment. Honoring such a foreign
// path would make the MCP server report another worktree's logs, so we ignore
// it and fall back to base/log, keeping each worktree's logs isolated.
func resolveMcpLogDir(base, flagValue, envValue string) string {
	if flagValue != "" {
		return anchorLogDir(base, flagValue)
	}
	if envValue != "" {
		p := anchorLogDir(base, envValue)
		if withinDir(base, p) {
			return p
		}
		// envValue points outside this worktree -> ignore it.
	}
	return filepath.Join(base, "log")
}

// anchorLogDir returns p cleaned when absolute, otherwise joins it onto base.
func anchorLogDir(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
}

// withinDir reports whether p is base itself or nested inside base.
func withinDir(base, p string) bool {
	base = filepath.Clean(base)
	p = filepath.Clean(p)
	if p == base {
		return true
	}
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
