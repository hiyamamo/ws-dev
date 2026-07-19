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

func TestReadServerPidStartTime(t *testing.T) {
	if processStartTime(os.Getpid()) == "" {
		t.Skip("ps -o lstart= not available")
	}
	path := filepath.Join(t.TempDir(), "server.pid")

	// A recorded start time that does not match the live process means the pid
	// was recycled: the file is stale and must be removed.
	content := strconv.Itoa(os.Getpid()) + "\nWed Jun 30 00:00:00 1993\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, alive, err := readServerPid(path)
	if err != nil || alive || pid != 0 {
		t.Fatalf("recycled pid: got pid=%d alive=%v err=%v, want 0/false/nil", pid, alive, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("recycled pid file should be removed, stat err=%v", err)
	}

	// A matching start time (what pidFileContents writes) stays alive.
	if err := os.WriteFile(path, []byte(pidFileContents(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, alive, err = readServerPid(path)
	if err != nil || !alive || pid != os.Getpid() {
		t.Fatalf("matching start time: got pid=%d alive=%v err=%v, want %d/true/nil", pid, alive, err, os.Getpid())
	}
}
