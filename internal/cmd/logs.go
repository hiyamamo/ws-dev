package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var (
		logDir string
		follow bool
		nLines int
	)
	c := &cobra.Command{
		Use:   "logs [<worktree>] [<name>]",
		Short: "List logs or tail a specific log (worktree defaults to cwd, then most recent server run)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := loadRepoCtx()
			if err != nil {
				return err
			}
			worktreeArg, name := "", ""
			if len(args) >= 1 {
				worktreeArg = args[0]
			}
			if len(args) == 2 {
				name = args[1]
			}
			// Precedence: explicit arg > most recent server run > cwd.
			// Only one server runs per repo, so the recorded run is almost
			// always the log set the user wants.
			if worktreeArg == "" {
				if l, rerr := readCurrentWorktree(); rerr == nil {
					worktreeArg = l
				}
			}
			_, dir, err := rc.resolveWorktree(worktreeArg)
			if err != nil {
				return err
			}
			logAbs := filepath.Join(dir, resolveLogDir(rc.Config, logDir))
			if name == "" {
				return listLogs(logAbs)
			}
			return tailLog(filepath.Join(logAbs, name+".log"), nLines, follow)
		},
	}
	c.Flags().StringVar(&logDir, "log-dir", "", "Log directory relative to the worktree (overrides config and $WS_DEV_LOG_DIR)")
	c.Flags().BoolVarP(&follow, "follow", "f", false, "Follow the log (tail -f)")
	c.Flags().IntVarP(&nLines, "lines", "n", 100, "Number of lines to show from the end")
	return c
}

// readCurrentWorktree returns the worktree name recorded by the most recent
// `ws-dev server` run.
func readCurrentWorktree() (string, error) {
	sdir, err := stateDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(sdir, worktreeFileName))
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", fmt.Errorf("no recent server run recorded")
	}
	return name, nil
}

func listLogs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type row struct {
		name  string
		size  int64
		mtime time.Time
	}
	rows := []row{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rows = append(rows, row{
			name:  strings.TrimSuffix(e.Name(), ".log"),
			size:  info.Size(),
			mtime: info.ModTime(),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].mtime.After(rows[j].mtime) })
	if len(rows) == 0 {
		fmt.Printf("(no logs in %s)\n", dir)
		return nil
	}
	fmt.Printf("# %s\n", dir)
	for _, r := range rows {
		fmt.Printf("  %-20s %10d  %s\n", r.name, r.size, r.mtime.Format(time.RFC3339))
	}
	return nil
}

func tailLog(path string, lines int, follow bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := printLastLines(f, lines); err != nil {
		return err
	}
	if !follow {
		return nil
	}
	for {
		n, err := io.Copy(os.Stdout, f)
		if err != nil {
			return err
		}
		if n == 0 {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func printLastLines(f *os.File, n int) error {
	const chunkSize = 64 * 1024
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	pos := size
	buf := []byte{}
	count := 0
	for pos > 0 && count <= n {
		readSize := int64(chunkSize)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		chunk := make([]byte, readSize)
		if _, err := f.ReadAt(chunk, pos); err != nil {
			return err
		}
		buf = append(chunk, buf...)
		count = 0
		for _, b := range buf {
			if b == '\n' {
				count++
			}
		}
	}
	text := string(buf)
	parts := strings.Split(text, "\n")
	start := 0
	if len(parts) > n+1 {
		start = len(parts) - n - 1
	}
	_, err = os.Stdout.WriteString(strings.Join(parts[start:], "\n"))
	return err
}
