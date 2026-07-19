package procman

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hiyamamo/ws-dev/internal/config"
)

// TestRunSetupRunsInOrderInDir verifies setup commands run sequentially in the
// worktree directory with template vars expanded.
func TestRunSetupRunsInOrderInDir(t *testing.T) {
	dir := t.TempDir()
	o := Opts{
		Cfg: &config.RepoConfig{Setup: []string{
			"printf one > step1",
			"printf {{.Worktree}} > step2",
		}},
		Worktree: "wt-x",
		Dir:      dir,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	}
	if err := RunSetup(o); err != nil {
		t.Fatalf("RunSetup: %v", err)
	}
	assertFile(t, filepath.Join(dir, "step1"), "one")
	assertFile(t, filepath.Join(dir, "step2"), "wt-x")
}

// TestRunSetupAbortsOnFailure verifies the first non-zero exit stops the
// sequence and later commands do not run.
func TestRunSetupAbortsOnFailure(t *testing.T) {
	dir := t.TempDir()
	o := Opts{
		Cfg: &config.RepoConfig{Setup: []string{
			"printf before > before",
			"false",
			"printf after > after",
		}},
		Dir:    dir,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	if err := RunSetup(o); err == nil {
		t.Fatal("expected error from failing setup command")
	}
	assertFile(t, filepath.Join(dir, "before"), "before")
	if _, err := os.Stat(filepath.Join(dir, "after")); !os.IsNotExist(err) {
		t.Error("command after a failure should not have run")
	}
}

func TestRunSetupEmpty(t *testing.T) {
	if err := RunSetup(Opts{Cfg: &config.RepoConfig{}}); err != nil {
		t.Fatalf("RunSetup with no setup: %v", err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}
