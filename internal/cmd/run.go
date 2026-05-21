package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/tasks"
)

func newRunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:                "run [<worktree>] <task> [args...]",
		Short:              "Run a configured task in a worktree (defaults to the repository root when the worktree is omitted)",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := loadRepoCtx()
			if err != nil {
				return err
			}
			worktreeArg, task, extra := splitRunArgs(rc, args)
			_, dir, err := rc.resolveWorktree(worktreeArg)
			if err != nil {
				return err
			}
			return tasks.Run(rc.Config, dir, task, extra)
		},
	}
	return c
}

// splitRunArgs decides whether the first arg is a worktree name or a task.
// `ws-dev run <task>` runs at the repository root; `ws-dev run <worktree>
// <task>` targets a named worktree. The first arg is treated as a worktree
// only when it is not itself a defined task and the following arg is.
func splitRunArgs(rc *repoCtx, args []string) (worktree, task string, extra []string) {
	if _, ok := rc.Config.Tasks[args[0]]; ok {
		return "", args[0], args[1:]
	}
	if len(args) >= 2 {
		if _, ok := rc.Config.Tasks[args[1]]; ok {
			return args[0], args[1], args[2:]
		}
	}
	return "", args[0], args[1:]
}
