package git

import "testing"

const samplePorcelain = `worktree /home/u/proj
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /home/u/proj/.claude/worktrees/feature-a
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feature-a

worktree /home/u/proj/.claude/worktrees/detached
HEAD 3333333333333333333333333333333333333333
detached
`

func TestParseWorktrees(t *testing.T) {
	wts := ParseWorktrees([]byte(samplePorcelain))
	if len(wts) != 3 {
		t.Fatalf("got %d worktrees, want 3", len(wts))
	}
	if wts[0].Path != "/home/u/proj" || wts[0].Branch != "main" {
		t.Errorf("wt[0] = %+v", wts[0])
	}
	if wts[1].Branch != "feature-a" {
		t.Errorf("wt[1].Branch = %q", wts[1].Branch)
	}
	if wts[2].Branch != "" {
		t.Errorf("detached wt should have empty branch, got %q", wts[2].Branch)
	}
}

func TestMainRoot(t *testing.T) {
	wts := ParseWorktrees([]byte(samplePorcelain))
	if got := MainRoot(wts); got != "/home/u/proj" {
		t.Errorf("MainRoot = %q", got)
	}
	if got := MainRoot(nil); got != "" {
		t.Errorf("MainRoot(nil) = %q, want empty", got)
	}
}

func TestResolveWorktree(t *testing.T) {
	wts := ParseWorktrees([]byte(samplePorcelain))

	path, err := ResolveWorktree(wts, "feature-a")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/home/u/proj/.claude/worktrees/feature-a" {
		t.Errorf("path = %q", path)
	}

	if _, err := ResolveWorktree(wts, "missing"); err == nil {
		t.Error("expected error for unknown worktree")
	}

	// Ambiguous basename.
	dup := append(wts, Worktree{Path: "/elsewhere/feature-a"})
	if _, err := ResolveWorktree(dup, "feature-a"); err == nil {
		t.Error("expected error for ambiguous basename")
	}
}

func TestCurrentWorktree(t *testing.T) {
	wts := ParseWorktrees([]byte(samplePorcelain))

	// cwd inside feature-a should match feature-a, not the main root
	// (longest prefix wins).
	wt, ok := CurrentWorktree(wts, "/home/u/proj/.claude/worktrees/feature-a/sub/dir")
	if !ok || wt.Branch != "feature-a" {
		t.Errorf("CurrentWorktree = %+v, ok=%v", wt, ok)
	}

	// cwd in the main worktree (but not in any nested worktree).
	wt, ok = CurrentWorktree(wts, "/home/u/proj/app/models")
	if !ok || wt.Path != "/home/u/proj" {
		t.Errorf("CurrentWorktree main = %+v, ok=%v", wt, ok)
	}

	// Outside any worktree.
	if _, ok := CurrentWorktree(wts, "/tmp/other"); ok {
		t.Error("expected no match outside worktrees")
	}

	// Prefix must respect path boundaries: /home/u/proj-other is not inside /home/u/proj.
	if wt, ok := CurrentWorktree(wts, "/home/u/proj-other/x"); ok {
		t.Errorf("false prefix match: %+v", wt)
	}
}
