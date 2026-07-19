package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/config"
	"github.com/hiyamamo/ws-dev/internal/git"
)

// completeWorktrees is a cobra ValidArgsFunction for the optional <worktree>
// positional argument shared by commands like `server` and `logs`. It only
// fires on the first positional arg (the worktree); later args get no
// file completion. Worktree names come straight from git rather than the
// ws-dev config, so completion works even in an unconfigured repo, and any
// failure degrades to "no suggestions" rather than erroring.
func completeWorktrees(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	wts, err := git.Worktrees()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return worktreeNames(wts, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// worktreeNames returns the basenames of wts that start with prefix. It mirrors
// git.ResolveWorktree, which matches a worktree by its directory basename.
func worktreeNames(wts []git.Worktree, prefix string) []string {
	var names []string
	for _, wt := range wts {
		name := filepath.Base(wt.Path)
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names
}

// completeLogsArgs completes `logs [<worktree>] [<name>]`: worktree names for
// the first arg, log names from that worktree's log dir for the second.
func completeLogsArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeWorktrees(cmd, args, toComplete)
	}
	if len(args) != 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	rc, err := loadRepoCtx()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	_, dir, err := rc.resolveWorktree(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := os.ReadDir(filepath.Join(dir, resolveLogDir(rc.Config, "")))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		if name := strings.TrimSuffix(e.Name(), ".log"); strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeRunArgs completes `run [<worktree>] <task> [args...]`, mirroring
// splitRunArgs: the first arg may be a worktree or a task, and the second is a
// task only when the first named a worktree (not a task).
func completeRunArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc, err := loadRepoCtx()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	switch len(args) {
	case 0:
		names, _ := completeWorktrees(cmd, args, toComplete)
		return append(names, taskNames(rc.Config, toComplete)...), cobra.ShellCompDirectiveNoFileComp
	case 1:
		if _, isTask := rc.Config.Tasks[args[0]]; isTask {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return taskNames(rc.Config, toComplete), cobra.ShellCompDirectiveNoFileComp
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// taskNames returns the configured task names that start with prefix, sorted
// for stable suggestions (the Tasks map has no order).
func taskNames(cfg *config.RepoConfig, prefix string) []string {
	names := make([]string, 0, len(cfg.Tasks))
	for name := range cfg.Tasks {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
