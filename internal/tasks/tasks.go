package tasks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hiyamamo/ws-dev/internal/config"
)

// BuildArgv splits the task command into argv form, prepends exec_wrapper,
// and appends extra args. The task command is split by shell-style whitespace
// splitting (simple, not full shell parsing).
func BuildArgv(cfg *config.RepoConfig, taskCmd string, extra []string) []string {
	parts := fields(taskCmd)
	argv := make([]string, 0, len(cfg.ExecWrapper)+len(parts)+len(extra))
	argv = append(argv, cfg.ExecWrapper...)
	argv = append(argv, parts...)
	argv = append(argv, extra...)
	return argv
}

// Run executes a configured task inside dir with stdio inherited.
// extraArgs are appended to the configured task command.
func Run(cfg *config.RepoConfig, dir, task string, extraArgs []string) error {
	cmdStr, ok := cfg.Tasks[task]
	if !ok {
		return fmt.Errorf("task %q not defined", task)
	}
	argv := BuildArgv(cfg, cmdStr, extraArgs)
	if len(argv) == 0 {
		return errors.New("empty task command")
	}
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = os.Environ()
	return c.Run()
}

// fields is a whitespace splitter that keeps simple quoted segments intact.
// It is intentionally minimal: supports "..." and '...' but no escaping.
func fields(s string) []string {
	var out []string
	var buf strings.Builder
	var quote byte
	flush := func() {
		if buf.Len() > 0 {
			out = append(out, buf.String())
			buf.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				buf.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ' ' || c == '\t':
			flush()
		default:
			buf.WriteByte(c)
		}
	}
	flush()
	return out
}
