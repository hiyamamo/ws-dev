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
		Use:                "run [<label>] <task> [args...]",
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

// splitRunArgs decides whether the first arg is a label or a task. When cwd is
// inside a repo dir, the first arg is treated as the task (label inferred from
// cwd). If the first arg isn't a defined task and args[1] is, fall back to the
// explicit `<label> <task>` form.
func splitRunArgs(ws *workspace.Workspace, args []string) (label, task string, extra []string, err error) {
	if cwdLabel, ok := ws.LabelFromCwd(); ok {
		_, firstIsTask := ws.Config.Tasks[args[0]]
		if firstIsTask || len(args) < 2 {
			return cwdLabel, args[0], args[1:], nil
		}
		if _, secondIsTask := ws.Config.Tasks[args[1]]; secondIsTask {
			return args[0], args[1], args[2:], nil
		}
		return cwdLabel, args[0], args[1:], nil
	}
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("usage: ws-dev run <label> <task> [args...] (label may be omitted when run inside repos/<repo-name>-<label>/)")
	}
	return args[0], args[1], args[2:], nil
}
