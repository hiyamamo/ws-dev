package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/tasks"
	"github.com/hiyamamo/ws-dev/internal/workspace"
)

func newRunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:                "run <task> [<label>] [args...]",
		Short:              "Run a configured task inside repos/<repo-name>-<label>/ (label is inferred from cwd when omitted)",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			ws, err := workspace.FindFromCwd()
			if err != nil {
				return err
			}
			label, task, extra, err := splitRunArgs(ws, args)
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

// splitRunArgs parses the `<task> [<label>] [args...]` form. The first arg is
// always the task. When cwd is inside a repo dir, the label is inferred from
// cwd unless args[1] names an existing repo dir (explicit override). When cwd
// is outside a repo dir, args[1] is required and used as the label.
func splitRunArgs(ws *workspace.Workspace, args []string) (label, task string, extra []string, err error) {
	task = args[0]
	if cwdLabel, ok := ws.LabelFromCwd(); ok {
		if len(args) >= 2 {
			if _, statErr := os.Stat(ws.RepoDir(args[1])); statErr == nil {
				return args[1], task, args[2:], nil
			}
		}
		return cwdLabel, task, args[1:], nil
	}
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("usage: ws-dev run <task> <label> [args...] (label may be omitted when run inside repos/<repo-name>-<label>/)")
	}
	return args[1], task, args[2:], nil
}
