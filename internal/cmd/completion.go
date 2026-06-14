package cmd

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
