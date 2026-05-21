package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hiyamamo/ws-dev/internal/config"
	"github.com/hiyamamo/ws-dev/internal/git"
)

// repoCtx bundles the resolved per-repo configuration and the repository's
// worktree list. It replaces the old workspace lookup: instead of finding a
// ws-dev.yml on disk, we identify the repo by its origin remote and match it
// against ~/.config/ws-dev/config.yml.
type repoCtx struct {
	Remote    string
	Root      string // main worktree root (absolute)
	Worktrees []git.Worktree
	Config    *config.RepoConfig
}

// loadRepoCtx resolves the current repository's configuration. It errors with
// actionable guidance when the repo is unconfigured.
func loadRepoCtx() (*repoCtx, error) {
	remote, err := git.Remote()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository with an 'origin' remote: %w", err)
	}
	wts, err := git.Worktrees()
	if err != nil {
		return nil, err
	}
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("%w\n(run `ws-dev init` to create %s)", err, cfgPath)
	}
	rc, ok := cfg.Lookup(remote)
	if !ok {
		return nil, fmt.Errorf("no config entry for remote %q in %s\n(run `ws-dev init` for the key to add)", remote, cfgPath)
	}
	return &repoCtx{Remote: remote, Root: git.MainRoot(wts), Worktrees: wts, Config: rc}, nil
}

// resolveWorktree maps a worktree name to its name and absolute directory.
// When name is empty it defaults to the repository's main worktree (root).
func (c *repoCtx) resolveWorktree(name string) (wtName, dir string, err error) {
	if name == "" {
		return filepath.Base(c.Root), c.Root, nil
	}
	dir, err = git.ResolveWorktree(c.Worktrees, name)
	if err != nil {
		return "", "", err
	}
	return name, dir, nil
}

// stateDir returns <git-common-dir>/ws-dev, creating it. The shared git dir is
// identical across worktrees and is never committed, so per-repo state lives
// there without polluting any working tree.
func stateDir() (string, error) {
	common, err := git.CommonDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(common, "ws-dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// resolveLogDir applies precedence flag > env > config > "log".
func resolveLogDir(cfg *config.RepoConfig, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("WS_DEV_LOG_DIR"); v != "" {
		return v
	}
	if cfg.LogDir != "" {
		return cfg.LogDir
	}
	return "log"
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
