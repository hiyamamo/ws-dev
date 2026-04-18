package cmd

import (
	"fmt"
	"os"

	"github.com/hiyamamo/ws-dev/internal/tasks"
	"github.com/hiyamamo/ws-dev/internal/workspace"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:                "run <label> <task> [args...]",
		Short:              "Run a configured task inside repos/<repo-name>-<label>/",
		Args:               cobra.MinimumNArgs(2),
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			label, task, extra := args[0], args[1], args[2:]
			ws, err := workspace.FindFromCwd()
			if err != nil {
				return err
			}
			repoDir := ws.RepoDir(label)
			if _, err := os.Stat(repoDir); err != nil {
				return fmt.Errorf("repo dir %s not found", repoDir)
			}
			return tasks.Run(ws.Config, repoDir, task, extra)
		},
	}
	return c
}
