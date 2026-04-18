package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed templates/ws-dev.yml.tmpl
var initConfigTemplate []byte

const gitignoreContents = `# ws-dev workspace
/.ws-dev/
/repos/
# Symlinked environment files (replace or extend as needed).
/links/.envrc
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <name>",
		Short: "Create a new workspace directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := scaffold(name); err != nil {
				return err
			}
			fmt.Printf("Initialized workspace at %s\n", name)
			fmt.Printf("Next steps:\n")
			fmt.Printf("  1. Edit %s/ws-dev.yml (set repo URL, processes, tasks)\n", name)
			fmt.Printf("  2. Place shared env files under %s/links/\n", name)
			fmt.Printf("  3. cd %s && ws-dev clone <label>\n", name)
			return nil
		},
	}
}

func scaffold(name string) error {
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("%s already exists", name)
	}
	if err := os.MkdirAll(filepath.Join(name, "repos"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(name, "links"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(name, "ws-dev.yml"), initConfigTemplate, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(name, ".gitignore"), []byte(gitignoreContents), 0o644); err != nil {
		return err
	}
	return nil
}
