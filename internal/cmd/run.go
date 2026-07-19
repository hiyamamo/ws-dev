package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/tasks"
)

func newRunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "run [<worktree>] <task> [args...]",
		Short: "Run a configured task in a worktree (defaults to the repository root when the worktree is omitted)",
		Long: `Run a configured task inside a worktree.

Arguments are positional and ordered: the worktree name comes first, then
the task name, then any extra args passed through to the task. The worktree
name is optional; when omitted the task runs at the repository root and the
first argument is the task name.`,
		Example: `  # omit the worktree: the task runs at the repository root
  ws-dev run console

  # target a worktree: worktree name first, then task name
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
			worktreeArg, task, extra := splitRunArgs(rc, args)
			worktree, dir, err := rc.resolveWorktree(worktreeArg)
			if err != nil {
				return err
			}
			// Tasks get the same template vars and WS_DEV_* environment as
			// processes and setup commands.
			v := tasks.Vars{
				Worktree: worktree,
				Root:     rc.Root,
				Dir:      dir,
				PortBase: resolvePortBase(0),
			}
			logAbs := filepath.Join(dir, resolveLogDir(rc.Config, ""))
			return tasks.Run(rc.Config, task, extra, v, logAbs)
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
