package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferRepoName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/C-FO/freee-invoice":     "freee-invoice",
		"https://github.com/C-FO/freee-invoice.git": "freee-invoice",
		"git@github.com:C-FO/freee-invoice.git":     "freee-invoice",
		"git@github.com:owner/repo":                 "repo",
		"https://example.com/a/b/c":                 "c",
	}
	for in, want := range cases {
		if got := inferRepoName(in); got != want {
			t.Errorf("inferRepoName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ws-dev.yml")
	src := `repo: https://github.com/owner/repo
log_dir: logs
processes:
  web:
    cmd: "echo hi"
    env:
      PORT: "3000"
tasks:
  hello: "echo hi"
links:
  - .envrc
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.RepoName() != "repo" {
		t.Errorf("RepoName = %q, want repo", c.RepoName())
	}
	if c.LogDir != "logs" {
		t.Errorf("LogDir = %q, want logs", c.LogDir)
	}
	if c.Processes["web"].Env["PORT"] != "3000" {
		t.Errorf("env PORT not parsed: %+v", c.Processes["web"])
	}
	if c.Tasks["hello"] != "echo hi" {
		t.Errorf("tasks.hello missing: %+v", c.Tasks)
	}
	if len(c.Links) != 1 || c.Links[0] != ".envrc" {
		t.Errorf("links = %+v", c.Links)
	}
}

func TestLoadMissingRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ws-dev.yml")
	if err := os.WriteFile(path, []byte("log_dir: logs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing repo")
	}
}
