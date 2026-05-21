package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/git"
	"github.com/hiyamamo/ws-dev/internal/tasks"
)

func newRunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "run [<worktree>] <task> [args...]",
		Short: "Run a configured task inside a worktree (worktree is inferred from cwd when omitted)",
		Long: `Run a configured task inside a worktree.

Arguments are positional and ordered: the worktree name comes first, then
the task name, then any extra args passed through to the task. The worktree
name is optional; when run from inside a worktree it may be omitted, in which
case the first argument is the task name.`,
		Example: `  # inside a worktree: first arg is the task name
  ws-dev run console

  # from anywhere: worktree name first, then task name
  ws-dev run branch-a console

  # extra args after the task name are passed through to it
  ws-dev run branch-a migrate VERSION=20240101`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := loadRepoCtx()
			if err != nil {
				return err
			}
			worktreeArg, task, extra, err := splitRunArgs(rc, args)
			if err != nil {
				return err
			}
			_, dir, err := rc.resolveWorktree(worktreeArg)
			if err != nil {
				return err
			}
			return tasks.Run(rc.Config, dir, task, extra)
		},
	}
	return c
}

// splitRunArgs decides whether the first arg is a worktree name or a task. When
// cwd is inside a worktree, the first arg is treated as the task (worktree
// inferred from cwd). If the first arg isn't a defined task and args[1] is, fall
// back to the explicit `<worktree> <task>` form.
func splitRunArgs(rc *repoCtx, args []string) (worktree, task string, extra []string, err error) {
	cwdInWorktree := false
	if cwd, e := os.Getwd(); e == nil {
		_, cwdInWorktree = git.CurrentWorktree(rc.Worktrees, cwd)
	}
	if cwdInWorktree {
		_, firstIsTask := rc.Config.Tasks[args[0]]
		if firstIsTask || len(args) < 2 {
			return "", args[0], args[1:], nil
		}
		if _, secondIsTask := rc.Config.Tasks[args[1]]; secondIsTask {
			return args[0], args[1], args[2:], nil
		}
		return "", args[0], args[1:], nil
	}
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("usage: ws-dev run <worktree> <task> [args...] (worktree may be omitted when run inside a worktree)")
	}
	return args[0], args[1], args[2:], nil
}
