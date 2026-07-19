package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/hiyamamo/ws-dev/internal/config"
)

// Vars are template variables exposed to process, task, and setup command
// strings.
type Vars struct {
	Worktree string // worktree name (basename)
	Root     string // main worktree root (absolute)
	Dir      string // worktree directory where the command runs (absolute)
	PortBase int
}

// Expand evaluates a command string as a text/template against the worktree
// vars ({{.Worktree}} / {{.PortBase}} / {{.Root}} / {{.Dir}}). It is shared by
// process cmds, tasks, and setup commands.
func Expand(cmd string, v Vars) (string, error) {
	tmpl, err := template.New("cmd").Parse(cmd)
	if err != nil {
		return "", fmt.Errorf("parse cmd template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", fmt.Errorf("expand cmd template: %w", err)
	}
	return buf.String(), nil
}

// Env returns the WS_DEV_* environment variables derived from vars, shared by
// processes, tasks, and setup commands. logDir is the absolute log directory.
func Env(v Vars, logDir string) []string {
	return []string{
		"WS_DEV_WORKTREE=" + v.Worktree,
		"WS_DEV_ROOT=" + v.Root,
		"WS_DEV_DIR=" + v.Dir,
		fmt.Sprintf("WS_DEV_PORT_BASE=%d", v.PortBase),
		"WS_DEV_LOG_DIR=" + logDir,
	}
}

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

// Run executes a configured task inside v.Dir with stdio inherited. The task
// command is template-expanded with the same vars as process cmds, and the
// same WS_DEV_* environment is exported. extraArgs are appended to the
// configured task command.
func Run(cfg *config.RepoConfig, task string, extraArgs []string, v Vars, logDir string) error {
	cmdStr, ok := cfg.Tasks[task]
	if !ok {
		return fmt.Errorf("task %q not defined", task)
	}
	expanded, err := Expand(cmdStr, v)
	if err != nil {
		return fmt.Errorf("task %q: %w", task, err)
	}
	argv := BuildArgv(cfg, expanded, extraArgs)
	if len(argv) == 0 {
		return errors.New("empty task command")
	}
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = v.Dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), Env(v, logDir)...)
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
