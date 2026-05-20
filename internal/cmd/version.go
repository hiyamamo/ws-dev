package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// These are injected via -ldflags by GoReleaser on tagged releases. For
// `go install`/`go build` they stay at their defaults and are filled in from
// the embedded build info instead (see resolveBuildInfo).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, c, d := resolveBuildInfo()
			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"ws-dev %s (commit %s, built %s, %s/%s)\n",
				v, c, d, runtime.GOOS, runtime.GOARCH)
			return err
		},
	}
}

// resolveBuildInfo prefers the ldflags-injected values (GoReleaser). When those
// are absent — i.e. installed via `go install module@version` or built locally —
// it falls back to runtime/debug build info so the module version and VCS stamp
// are still reported instead of the "dev"/"none" placeholders.
func resolveBuildInfo() (v, c, d string) {
	v, c, d = version, commit, date
	if v != "dev" {
		return // ldflags-injected release build
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	// `go install pkg@v0.2.0` records the version; local `go build` records
	// "(devel)", which we leave as "dev".
	if mv := info.Main.Version; mv != "" && mv != "(devel)" {
		v = mv
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				c = s.Value[:7]
			} else if s.Value != "" {
				c = s.Value
			}
		case "vcs.time":
			d = s.Value
		}
	}
	return
}
