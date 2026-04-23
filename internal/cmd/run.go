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
		Use:                "run <task> [args...] [<label>]",
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

// splitRunArgs parses the `<task> [args...] [<label>]` form. The first arg is
// always the task. If the last arg names an existing repos/<repo-name>-<arg>/
// directory, it is taken as the label and the args between become
// pass-through. Otherwise the label is inferred from cwd (when inside a repo
// dir) and all remaining args become pass-through.
func splitRunArgs(ws *workspace.Workspace, args []string) (label, task string, extra []string, err error) {
	task = args[0]
	if len(args) >= 2 {
		last := args[len(args)-1]
		if _, statErr := os.Stat(ws.RepoDir(last)); statErr == nil {
			return last, task, args[1 : len(args)-1], nil
		}
	}
	if cwdLabel, ok := ws.LabelFromCwd(); ok {
		return cwdLabel, task, args[1:], nil
	}
	return "", "", nil, fmt.Errorf("usage: ws-dev run <task> [args...] <label> (label may be omitted when run inside repos/<repo-name>-<label>/)")
}
