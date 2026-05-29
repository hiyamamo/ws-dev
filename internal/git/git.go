// Package git wraps the handful of git commands ws-dev needs and provides
// pure helpers for interpreting their output.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path   string
	Head   string
	Branch string // short name (refs/heads/ stripped); empty when detached
}

// Remote returns the URL of the "origin" remote.
func Remote() (string, error) {
	out, err := run("remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// CommonDir returns the absolute path of the shared git directory
// (`git rev-parse --git-common-dir`). It is identical across all worktrees of
// a repository, which makes it a good home for per-repo state.
func CommonDir() (string, error) {
	out, err := run("rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	dir := strings.TrimSpace(out)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// Toplevel returns the absolute path of the working tree that contains the
// current directory (`git rev-parse --show-toplevel`). For a worktree created
// by `git worktree add` this is that worktree's root, not the main worktree.
func Toplevel() (string, error) {
	out, err := run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	dir := strings.TrimSpace(out)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// Worktrees returns all worktrees of the current repository.
func Worktrees() ([]Worktree, error) {
	out, err := run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return ParseWorktrees([]byte(out)), nil
}

func run(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return stdout.String(), nil
}

// ParseWorktrees parses the output of `git worktree list --porcelain`.
// Blocks are separated by blank lines; the first block is the main worktree.
func ParseWorktrees(porcelain []byte) []Worktree {
	var out []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, raw := range strings.Split(string(porcelain), "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			flush()
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			cur = &Worktree{Path: val}
		case "HEAD":
			if cur != nil {
				cur.Head = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		}
	}
	flush()
	return out
}

// MainRoot returns the path of the main worktree (the first entry).
func MainRoot(wts []Worktree) string {
	if len(wts) == 0 {
		return ""
	}
	return wts[0].Path
}

// ResolveWorktree returns the path of the worktree whose directory basename
// equals name. It errors when no match or more than one match is found.
func ResolveWorktree(wts []Worktree, name string) (string, error) {
	var matches []string
	for _, wt := range wts {
		if filepath.Base(wt.Path) == name {
			matches = append(matches, wt.Path)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no worktree named %q (create one with `claude -w %s` or `git worktree add`)", name, name)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("worktree name %q is ambiguous: %s", name, strings.Join(matches, ", "))
	}
}
