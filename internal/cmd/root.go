package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ws-dev",
		Short: "Parallel workspace manager for multi-clone development",
		Long: `ws-dev manages multiple clones of the same repository under repos/,
with shared symlinked env files, a Procfile-like process manager,
user-defined tasks, and a built-in MCP server for log operations.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		newInitCmd(),
		newCloneCmd(),
		newLinkCmd(),
		newUnlinkCmd(),
		newServerCmd(),
		newLogsCmd(),
		newRunCmd(),
		newTasksCmd(),
		newMcpCmd(),
		newVersionCmd(),
	)
	return root
}
