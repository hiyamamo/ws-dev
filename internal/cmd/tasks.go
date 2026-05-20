package cmd

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newTasksCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tasks",
		Short: "List tasks defined for the current repo in ~/.config/ws-dev/config.yml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rc, err := loadRepoCtx()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(rc.Config.Tasks))
			for name := range rc.Config.Tasks {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no tasks defined for this repo")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, name := range names {
				_, _ = fmt.Fprintf(w, "%s\t%s\n", name, rc.Config.Tasks[name])
			}
			return w.Flush()
		},
	}
	return c
}
