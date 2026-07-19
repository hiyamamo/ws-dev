package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ws-dev",
		Short: "Run apps and tasks inside git worktrees",
		Long: `ws-dev runs a Procfile-like process manager, user-defined tasks, and a
built-in MCP server for log operations inside git worktrees. Per-repository
settings live in ~/.config/ws-dev/config.yml, keyed by the repo's git remote.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		newInitCmd(),
		newServerCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newRunCmd(),
		newTasksCmd(),
		newMcpCmd(),
		newUpdateCmd(),
		newVersionCmd(),
	)
	return root
}
