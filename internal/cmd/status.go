package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the ws-dev server is running and for which worktree",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			sdir, err := stateDir()
			if err != nil {
				return err
			}
			pid, alive, err := readServerPid(filepath.Join(sdir, pidFileName))
			if err != nil {
				return err
			}
			// The recorded worktree is informational either way: while running
			// it names the active server's worktree; otherwise the most recent
			// run (the same default `ws-dev logs` uses).
			worktree, _ := readCurrentWorktree()
			if !alive {
				fmt.Println("No ws-dev server running")
				if worktree != "" {
					fmt.Printf("Last worktree: %s\n", worktree)
				}
				return nil
			}
			fmt.Printf("ws-dev server running (pid %d)\n", pid)
			if worktree != "" {
				fmt.Printf("Worktree: %s\n", worktree)
			}
			return nil
		},
	}
}
