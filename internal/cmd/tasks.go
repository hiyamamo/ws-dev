package cmd

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/hiyamamo/ws-dev/internal/workspace"
	"github.com/spf13/cobra"
)

func newTasksCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tasks",
		Short: "List tasks defined in ws-dev.yml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, err := workspace.FindFromCwd()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(ws.Config.Tasks))
			for name := range ws.Config.Tasks {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no tasks defined in ws-dev.yml")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, name := range names {
				fmt.Fprintf(w, "%s\t%s\n", name, ws.Config.Tasks[name])
			}
			return w.Flush()
		},
	}
	return c
}
