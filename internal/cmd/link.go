package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/links"
	"github.com/hiyamamo/ws-dev/internal/workspace"
)

func newLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <label>",
		Short: "Symlink files from links/ into repos/<repo-name>-<label>/",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runLink(args[0], false)
		},
	}
}

func newUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <label>",
		Short: "Remove symlinks created by `link` from repos/<repo-name>-<label>/",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runLink(args[0], true)
		},
	}
}

func runLink(label string, undo bool) error {
	ws, err := workspace.FindFromCwd()
	if err != nil {
		return err
	}
	repoDir := ws.RepoDir(label)
	if _, err := os.Stat(repoDir); err != nil {
		return fmt.Errorf("repo dir %s not found (run `ws-dev clone %s` first)", repoDir, label)
	}
	if undo {
		return links.Unlink(repoDir, ws.Config.Links)
	}
	return links.Link(ws.LinksDir(), repoDir, ws.Config.Links)
}
