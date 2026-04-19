package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiyamamo/ws-dev/internal/config"
)

const ConfigFile = "ws-dev.yml"

type Workspace struct {
	Root   string
	Config *config.Config
}

// Find walks up from startDir looking for ws-dev.yml.
func Find(startDir string) (*Workspace, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(dir, ConfigFile)
		if _, err := os.Stat(candidate); err == nil {
			cfg, err := config.Load(candidate)
			if err != nil {
				return nil, err
			}
			return &Workspace{Root: dir, Config: cfg}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no %s found in %s or any parent", ConfigFile, startDir)
		}
		dir = parent
	}
}

// FindFromCwd is a convenience wrapper around Find using the current directory.
func FindFromCwd() (*Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return Find(cwd)
}

// RepoDir returns the absolute path to repos/<repo-name>-<label>.
func (w *Workspace) RepoDir(label string) string {
	return filepath.Join(w.Root, "repos", w.Config.RepoName()+"-"+label)
}

// LabelFromDir returns the label inferred from dir when dir is inside
// <root>/repos/<repo-name>-<label>/ (at any depth). The label must be
// non-empty.
func (w *Workspace) LabelFromDir(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	reposDir := filepath.Join(w.Root, "repos")
	prefix := w.Config.RepoName() + "-"
	cur := abs
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		if parent == reposDir {
			base := filepath.Base(cur)
			if !strings.HasPrefix(base, prefix) {
				return "", false
			}
			label := strings.TrimPrefix(base, prefix)
			if label == "" {
				return "", false
			}
			return label, true
		}
		cur = parent
	}
}

// LabelFromCwd is a convenience wrapper around LabelFromDir using the current
// directory.
func (w *Workspace) LabelFromCwd() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return w.LabelFromDir(cwd)
}

// LinksDir returns the absolute path to links/.
func (w *Workspace) LinksDir() string {
	return filepath.Join(w.Root, "links")
}

// StateDir returns the absolute path to .ws-dev/ and ensures it exists.
func (w *Workspace) StateDir() (string, error) {
	dir := filepath.Join(w.Root, ".ws-dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ResolveLogDir returns the log directory (relative to repo dir),
// applying the precedence: flag > env > config > default "log".
func (w *Workspace) ResolveLogDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("WS_DEV_LOG_DIR"); v != "" {
		return v
	}
	if w.Config.LogDir != "" {
		return w.Config.LogDir
	}
	return "log"
}
