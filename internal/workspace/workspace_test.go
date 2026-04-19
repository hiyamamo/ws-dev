package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAndLabelFromDir(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ConfigFile)
	src := "repo: https://github.com/owner/repo\n"
	if err := os.WriteFile(cfgPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	repoLabelDir := filepath.Join(root, "repos", "repo-branch-a")
	nestedDir := filepath.Join(repoLabelDir, "app", "models")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Find from a nested dir should locate ws-dev.yml at root.
	ws, err := Find(nestedDir)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ws.Root != root {
		t.Fatalf("Root = %q, want %q", ws.Root, root)
	}

	// LabelFromDir should infer "branch-a" from the repo dir itself.
	if got, ok := ws.LabelFromDir(repoLabelDir); !ok || got != "branch-a" {
		t.Fatalf("LabelFromDir(repoDir) = %q,%v, want branch-a,true", got, ok)
	}

	// LabelFromDir should infer the same from any nested subdir.
	if got, ok := ws.LabelFromDir(nestedDir); !ok || got != "branch-a" {
		t.Fatalf("LabelFromDir(nested) = %q,%v, want branch-a,true", got, ok)
	}

	// Outside repos/ — no label.
	if _, ok := ws.LabelFromDir(root); ok {
		t.Fatalf("LabelFromDir(root) should not match")
	}

	// Inside repos/ but not under <repo>-<label>/ — no label.
	other := filepath.Join(root, "repos", "stray", "sub")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := ws.LabelFromDir(other); ok {
		t.Fatalf("LabelFromDir(stray) should not match (no %q prefix)", ws.Config.RepoName()+"-")
	}

	// Empty label — no match.
	emptyLabel := filepath.Join(root, "repos", "repo-")
	if err := os.MkdirAll(emptyLabel, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := ws.LabelFromDir(emptyLabel); ok {
		t.Fatalf("LabelFromDir(empty label) should not match")
	}
}
