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
		Use:   "link [<label>]",
		Short: "Symlink files from links/ into repos/<repo-name>-<label>/ (label is inferred from cwd when omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runLink(firstArg(args), false)
		},
	}
}

func newUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink [<label>]",
		Short: "Remove symlinks created by `link` from repos/<repo-name>-<label>/ (label is inferred from cwd when omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runLink(firstArg(args), true)
		},
	}
}

func runLink(label string, undo bool) error {
	ws, err := workspace.FindFromCwd()
	if err != nil {
		return err
	}
	label, err = resolveLabel(ws, label)
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

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// resolveLabel returns the explicit label if non-empty; otherwise it tries to
// infer the label from the current working directory.
func resolveLabel(ws *workspace.Workspace, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if label, ok := ws.LabelFromCwd(); ok {
		return label, nil
	}
	return "", fmt.Errorf("label not specified and current directory is not under repos/%s-<label>/", ws.Config.RepoName())
}
