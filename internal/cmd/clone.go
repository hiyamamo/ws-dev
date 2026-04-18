package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/workspace"
)

func newCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <label>",
		Short: "Clone the configured repo into repos/<repo-name>-<label>/",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			label := args[0]
			ws, err := workspace.FindFromCwd()
			if err != nil {
				return err
			}
			dest := ws.RepoDir(label)
			if _, err := os.Stat(dest); err == nil {
				return fmt.Errorf("%s already exists", dest)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			git := exec.Command("git", "clone", ws.Config.Repo, dest)
			git.Stdout = os.Stdout
			git.Stderr = os.Stderr
			if err := git.Run(); err != nil {
				return fmt.Errorf("git clone failed: %w", err)
			}
			fmt.Printf("Cloned into %s\n", dest)
			return nil
		},
	}
}
