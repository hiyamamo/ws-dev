package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Process struct {
	Cmd string            `yaml:"cmd"`
	Env map[string]string `yaml:"env,omitempty"`
}

type Config struct {
	Repo        string             `yaml:"repo"`
	LogDir      string             `yaml:"log_dir,omitempty"`
	ExecWrapper []string           `yaml:"exec_wrapper,omitempty"`
	Processes   map[string]Process `yaml:"processes,omitempty"`
	Tasks       map[string]string  `yaml:"tasks,omitempty"`
	Links       []string           `yaml:"links,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Repo == "" {
		return nil, fmt.Errorf("%s: repo is required", path)
	}
	return c, nil
}

// RepoName infers a directory name from the repo URL.
// e.g. https://github.com/C-FO/freee-invoice -> "freee-invoice"
//
//	git@github.com:C-FO/freee-invoice.git -> "freee-invoice"
func (c *Config) RepoName() string {
	return inferRepoName(c.Repo)
}

func inferRepoName(repo string) string {
	s := strings.TrimSuffix(repo, ".git")
	if u, err := url.Parse(s); err == nil && u.Path != "" {
		s = u.Path
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	return path.Base(s)
}
