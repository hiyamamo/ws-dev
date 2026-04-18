package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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
			logDir := resolveMcpLogDir(logDirFlag)
			if !filepath.IsAbs(logDir) {
				logDir = filepath.Join(cwd, logDir)
			}
			return mcp.NewServer(logDir).Run()
		},
	}
	c.Flags().StringVar(&logDirFlag, "log-dir", "", "Log directory (overrides $WS_DEV_LOG_DIR)")
	return c
}

// resolveMcpLogDir picks the log dir: flag > env > "log".
// The MCP server runs inside a single repo dir (cwd), so we don't consult
// ws-dev.yml from here — the env var lets the launcher inject the right value.
func resolveMcpLogDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("WS_DEV_LOG_DIR"); v != "" {
		return v
	}
	return "log"
}
