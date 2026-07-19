package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTruncateLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "web.log")
	otherPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(logPath, []byte("old output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := truncateLogs(dir); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(logPath); err != nil || info.Size() != 0 {
		t.Errorf("web.log should be empty, size=%d err=%v", info.Size(), err)
	}
	if data, err := os.ReadFile(otherPath); err != nil || string(data) != "keep me\n" {
		t.Errorf("non-log file must be untouched, got %q err=%v", data, err)
	}

	if err := truncateLogs(filepath.Join(dir, "missing")); err != nil {
		t.Errorf("missing dir should be a no-op, got %v", err)
	}
}
