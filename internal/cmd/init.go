package cmd

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hiyamamo/ws-dev/internal/config"
	"github.com/hiyamamo/ws-dev/internal/git"
)

//go:embed templates/config.yml.tmpl
var initConfigTemplate []byte

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create ~/.config/ws-dev/config.yml and print the key for the current repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			path := config.DefaultPath()

			created, err := ensureConfig(path)
			if err != nil {
				return err
			}
			if created {
				_, _ = fmt.Fprintf(out, "Created %s\n", path)
			} else {
				_, _ = fmt.Fprintf(out, "Config already exists at %s\n", path)
			}

			// When run inside a git repo, show the key to register.
			if remote, err := git.Remote(); err == nil {
				key := config.NormalizeRemote(remote)
				_, _ = fmt.Fprintf(out, "\nThis repo's remote: %s\n", remote)
				_, _ = fmt.Fprintf(out, "Add an entry under `repos:` keyed by:\n\n  %s:\n    processes:\n      web:\n        cmd: \"...\"\n", key)
			}
			return nil
		},
	}
}

// ensureConfig writes the template to path if it does not already exist,
// creating parent directories. It returns whether a new file was created.
func ensureConfig(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, initConfigTemplate, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
