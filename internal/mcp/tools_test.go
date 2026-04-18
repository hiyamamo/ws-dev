package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLog(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListLogs(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "web.log", "hello\n")
	writeLog(t, dir, "worker.log", "work\n")
	writeLog(t, dir, "ignore.txt", "nope")
	logs, err := ListLogs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("want 2 logs, got %d: %+v", len(logs), logs)
	}
	names := map[string]bool{logs[0].Name: true, logs[1].Name: true}
	if !names["web"] || !names["worker"] {
		t.Errorf("missing entries: %+v", logs)
	}
}

func TestTailLog(t *testing.T) {
	dir := t.TempDir()
	lines := []string{"a", "b", "c", "d", "e"}
	writeLog(t, dir, "web.log", strings.Join(lines, "\n")+"\n")
	r, err := TailLog(dir, "web", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, "d") || !strings.Contains(r.Content, "e") {
		t.Errorf("tail did not return last lines: %q", r.Content)
	}
	if strings.Contains(r.Content, "a") {
		t.Errorf("tail returned too many lines: %q", r.Content)
	}
}

func TestSearchLog(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "web.log", "info ok\nERROR bad\nwarn\nERROR again\n")
	res, err := SearchLog(dir, "web", "ERROR", SearchOpts{MaxMatches: 10, Context: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalMatches != 2 {
		t.Errorf("total matches = %d, want 2", res.TotalMatches)
	}
	if len(res.Matches) != 2 {
		t.Errorf("shown matches = %d, want 2", len(res.Matches))
	}
	if len(res.Matches[0].Before) != 1 || res.Matches[0].Before[0].Line != "info ok" {
		t.Errorf("context before not captured: %+v", res.Matches[0])
	}
}

func TestSearchLogIgnoreCase(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "web.log", "Error\nerror\nERROR\n")
	res, err := SearchLog(dir, "web", "error", SearchOpts{MaxMatches: 10, IgnoreCase: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalMatches != 3 {
		t.Errorf("ignore-case: got %d, want 3", res.TotalMatches)
	}
}

func TestTruncateLog(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "web.log", "lots of content\n")
	r, err := TruncateLog(dir, "web")
	if err != nil {
		t.Fatal(err)
	}
	if r.BytesFreed != int64(len("lots of content\n")) {
		t.Errorf("bytes freed = %d, want %d", r.BytesFreed, len("lots of content\n"))
	}
	info, err := os.Stat(filepath.Join(dir, "web.log"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("log not truncated: size=%d", info.Size())
	}
}
