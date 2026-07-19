package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadServerPid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")

	pid, alive, err := readServerPid(path)
	if err != nil || alive || pid != 0 {
		t.Fatalf("missing file: got pid=%d alive=%v err=%v, want 0/false/nil", pid, alive, err)
	}

	if err := os.WriteFile(path, []byte("not-a-pid"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, alive, err = readServerPid(path)
	if err != nil || alive || pid != 0 {
		t.Fatalf("malformed file: got pid=%d alive=%v err=%v, want 0/false/nil", pid, alive, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("malformed pid file should be removed, stat err=%v", err)
	}

	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, alive, err = readServerPid(path)
	if err != nil || !alive || pid != os.Getpid() {
		t.Fatalf("own pid: got pid=%d alive=%v err=%v, want %d/true/nil", pid, alive, err, os.Getpid())
	}
}
