package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:owner/repo.git":       "github.com/owner/repo",
		"git@github.com:owner/repo":           "github.com/owner/repo",
		"https://github.com/owner/repo":       "github.com/owner/repo",
		"https://github.com/owner/repo.git":   "github.com/owner/repo",
		"https://github.com/owner/repo.git/":  "github.com/owner/repo",
		"ssh://git@github.com/owner/repo.git": "github.com/owner/repo",
		"git@github.com:Owner/Repo.git":       "github.com/owner/repo",
		"https://user@example.com/a/b/c":      "example.com/a/b/c",
	}
	for in, want := range cases {
		if got := NormalizeRemote(in); got != want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadAndLookup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	src := `repos:
  github.com/owner/repo:
    log_dir: logs
    exec_wrapper: ["mise", "exec", "--"]
    processes:
      web:
        cmd: "echo hi"
        env:
          PORT: "3000"
    tasks:
      hello: "echo hi"
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Lookup must match regardless of remote URL form.
	for _, remote := range []string{
		"https://github.com/owner/repo",
		"git@github.com:owner/repo.git",
		"ssh://git@github.com/owner/repo",
	} {
		rc, ok := c.Lookup(remote)
		if !ok {
			t.Fatalf("Lookup(%q) not found", remote)
		}
		if rc.LogDir != "logs" {
			t.Errorf("LogDir = %q, want logs", rc.LogDir)
		}
		if rc.Processes["web"].Env["PORT"] != "3000" {
			t.Errorf("env PORT not parsed: %+v", rc.Processes["web"])
		}
		if rc.Tasks["hello"] != "echo hi" {
			t.Errorf("tasks.hello missing: %+v", rc.Tasks)
		}
		if len(rc.ExecWrapper) != 3 {
			t.Errorf("exec_wrapper = %+v", rc.ExecWrapper)
		}
	}

	if _, ok := c.Lookup("git@github.com:other/missing.git"); ok {
		t.Error("Lookup of unknown repo should fail")
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
