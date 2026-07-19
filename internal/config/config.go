package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Process is a single long-running process managed by `ws-dev server`.
type Process struct {
	Cmd string            `yaml:"cmd"`
	Env map[string]string `yaml:"env,omitempty"`
}

// RepoConfig holds the per-repository settings looked up by remote URL.
type RepoConfig struct {
	LogDir      string             `yaml:"log_dir,omitempty"`
	FreshLogs   bool               `yaml:"fresh_logs,omitempty"`
	ExecWrapper []string           `yaml:"exec_wrapper,omitempty"`
	Setup       []string           `yaml:"setup,omitempty"`
	Processes   map[string]Process `yaml:"processes,omitempty"`
	Tasks       map[string]string  `yaml:"tasks,omitempty"`
}

// Config is the top-level ~/.config/ws-dev/config.yml document. Each entry in
// Repos is keyed by a git remote URL (any form; matched after normalization).
type Config struct {
	Repos map[string]RepoConfig `yaml:"repos"`
}

// DefaultPath returns the config file path, honoring $WS_DEV_CONFIG and
// $XDG_CONFIG_HOME, falling back to ~/.config/ws-dev/config.yml.
func DefaultPath() string {
	if v := os.Getenv("WS_DEV_CONFIG"); v != "" {
		return v
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ws-dev", "config.yml")
}

// Load reads and parses the config file at path. Two repos keys that
// normalize to the same remote are rejected: Lookup matches after
// normalization, so such entries would be picked nondeterministically (map
// order) otherwise.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	seen := map[string]string{}
	for key := range c.Repos {
		norm := NormalizeRemote(key)
		if prev, ok := seen[norm]; ok {
			first, second := prev, key
			if second < first {
				first, second = second, first
			}
			return nil, fmt.Errorf("parse %s: repos keys %q and %q both refer to %q; keep one entry", path, first, second, norm)
		}
		seen[norm] = key
	}
	return c, nil
}

// Lookup returns the RepoConfig whose key normalizes to the same value as
// remote. Both the stored keys and remote are normalized before comparison.
func (c *Config) Lookup(remote string) (*RepoConfig, bool) {
	target := NormalizeRemote(remote)
	for key, rc := range c.Repos {
		if NormalizeRemote(key) == target {
			rc := rc
			return &rc, true
		}
	}
	return nil, false
}

// NormalizeRemote canonicalizes a git remote URL so that ssh (scp) and https
// forms of the same repository compare equal. e.g. both
//
//	git@github.com:owner/repo.git
//	https://github.com/owner/repo
//
// normalize to "github.com/owner/repo".
func NormalizeRemote(remote string) string {
	s := strings.TrimSpace(remote)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")

	// Drop scheme (https://, ssh://, git://, ...).
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Drop userinfo (user@host) when it precedes the path.
	if at := strings.Index(s, "@"); at >= 0 {
		if slash := strings.Index(s, "/"); slash < 0 || at < slash {
			s = s[at+1:]
		}
	}
	// scp-like "host:path" -> "host/path" (first colon before any slash).
	if colon := strings.Index(s, ":"); colon >= 0 {
		if slash := strings.Index(s, "/"); slash < 0 || colon < slash {
			s = s[:colon] + "/" + s[colon+1:]
		}
	}
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.ToLower(s)
}
