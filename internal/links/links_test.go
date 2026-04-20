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
	if err := Unlink(linksDir, repoDir, entries); err != nil {
		t.Fatal(err)
	}
	for _, rel := range entries {
		if _, err := os.Lstat(filepath.Join(repoDir, rel)); err == nil {
			t.Errorf("%s still exists after Unlink", rel)
		}
	}
}

func TestLinkContentsMode(t *testing.T) {
	ws := t.TempDir()
	linksDir := filepath.Join(ws, "links")
	repoDir := filepath.Join(ws, "repo")

	uploadsSrc := filepath.Join(linksDir, "storage", "uploads")
	if err := os.MkdirAll(uploadsSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsSrc, "a"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsSrc, "b"), []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}

	uploadsTarget := filepath.Join(repoDir, "storage", "uploads")
	if err := os.MkdirAll(uploadsTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	gitkeep := filepath.Join(uploadsTarget, ".gitkeep")
	if err := os.WriteFile(gitkeep, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{"storage/uploads/"}
	if err := Link(linksDir, repoDir, entries); err != nil {
		t.Fatalf("Link failed: %v", err)
	}

	// Parent directory must remain a real directory.
	dirInfo, err := os.Lstat(uploadsTarget)
	if err != nil {
		t.Fatalf("uploads target missing: %v", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("uploads target is a symlink, want real dir")
	}

	// .gitkeep stays as a regular file.
	if info, err := os.Lstat(gitkeep); err != nil {
		t.Fatalf(".gitkeep missing: %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf(".gitkeep became a symlink")
	}

	for _, name := range []string{"a", "b"} {
		p := filepath.Join(uploadsTarget, name)
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink", p)
		}
	}

	if data, err := os.ReadFile(filepath.Join(uploadsTarget, "a")); err != nil || string(data) != "A" {
		t.Errorf("a resolve failed: %q, %v", data, err)
	}

	if err := Unlink(linksDir, repoDir, entries); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := os.Lstat(filepath.Join(uploadsTarget, name)); err == nil {
			t.Errorf("%s still exists after Unlink", name)
		}
	}
	if _, err := os.Lstat(uploadsTarget); err != nil {
		t.Errorf("parent dir removed by Unlink: %v", err)
	}
	if _, err := os.Lstat(gitkeep); err != nil {
		t.Errorf(".gitkeep removed by Unlink: %v", err)
	}
}

func TestLinkContentsSkipsRealFileCollision(t *testing.T) {
	ws := t.TempDir()
	linksDir := filepath.Join(ws, "links")
	repoDir := filepath.Join(ws, "repo")

	srcDir := filepath.Join(linksDir, "shared")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a"), []byte("from-links"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(repoDir, "shared")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	collide := filepath.Join(targetDir, "a")
	if err := os.WriteFile(collide, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Link(linksDir, repoDir, []string{"shared/"}); err != nil {
		t.Fatalf("Link failed: %v", err)
	}

	info, err := os.Lstat(collide)
	if err != nil {
		t.Fatalf("collision file missing: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("real file was replaced with symlink")
	}
	data, err := os.ReadFile(collide)
	if err != nil || string(data) != "real" {
		t.Errorf("real file modified: %q, %v", data, err)
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
