package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFollowStep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web.log")
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var out strings.Builder
	wrote, err := followStep(f, &out)
	if err != nil || !wrote || out.String() != "one\n" {
		t.Fatalf("initial: wrote=%v err=%v out=%q", wrote, err, out.String())
	}

	// Nothing new: no write, no error.
	out.Reset()
	wrote, err = followStep(f, &out)
	if err != nil || wrote || out.String() != "" {
		t.Fatalf("no new data: wrote=%v err=%v out=%q", wrote, err, out.String())
	}

	// Appended data is picked up from the current offset.
	appendFile(t, path, "two\n")
	out.Reset()
	wrote, err = followStep(f, &out)
	if err != nil || !wrote || out.String() != "two\n" {
		t.Fatalf("append: wrote=%v err=%v out=%q", wrote, err, out.String())
	}

	// Truncation below the read offset restarts from the top instead of
	// waiting forever past the new EOF.
	if err := os.WriteFile(path, []byte("fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	wrote, err = followStep(f, &out)
	if err != nil || !wrote || out.String() != "fresh\n" {
		t.Fatalf("truncate: wrote=%v err=%v out=%q", wrote, err, out.String())
	}
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}
