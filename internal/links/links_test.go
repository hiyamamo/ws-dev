package links

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkAndUnlink(t *testing.T) {
	ws := t.TempDir()
	linksDir := filepath.Join(ws, "links")
	repoDir := filepath.Join(ws, "repo")
	if err := os.MkdirAll(filepath.Join(linksDir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Plain file
	if err := os.WriteFile(filepath.Join(linksDir, ".envrc"), []byte("export A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Nested file
	if err := os.WriteFile(filepath.Join(linksDir, ".claude", "settings.local.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Directory
	storageSrc := filepath.Join(linksDir, "storage")
	if err := os.MkdirAll(filepath.Join(storageSrc, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageSrc, "sub", "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing directory that should be replaced (like the old storage dir).
	if err := os.MkdirAll(filepath.Join(repoDir, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []string{".envrc", ".claude/settings.local.json", "storage"}
	if err := Link(linksDir, repoDir, entries); err != nil {
		t.Fatalf("Link failed: %v", err)
	}

	for _, rel := range entries {
		p := filepath.Join(repoDir, rel)
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink", p)
		}
	}

	// Verify content resolves through symlink
	data, err := os.ReadFile(filepath.Join(repoDir, ".envrc"))
	if err != nil || string(data) != "export A=1\n" {
		t.Errorf(".envrc resolve failed: %q, %v", data, err)
	}

	// Unlink should remove all symlinks.
	if err := Unlink(repoDir, entries); err != nil {
		t.Fatal(err)
	}
	for _, rel := range entries {
		if _, err := os.Lstat(filepath.Join(repoDir, rel)); err == nil {
			t.Errorf("%s still exists after Unlink", rel)
		}
	}
}

func TestLinkReplacesExistingSymlink(t *testing.T) {
	ws := t.TempDir()
	linksDir := filepath.Join(ws, "links")
	repoDir := filepath.Join(ws, "repo")
	if err := os.MkdirAll(linksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linksDir, ".envrc"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing stale symlink
	if err := os.Symlink("/nowhere", filepath.Join(repoDir, ".envrc")); err != nil {
		t.Fatal(err)
	}
	if err := Link(linksDir, repoDir, []string{".envrc"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoDir, ".envrc"))
	if err != nil || string(data) != "v2" {
		t.Errorf("stale symlink not replaced: %q, %v", data, err)
	}
}
