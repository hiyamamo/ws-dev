package cmd

import (
	"reflect"
	"testing"

	"github.com/hiyamamo/ws-dev/internal/git"
)

func TestWorktreeNames(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/home/u/proj"},
		{Path: "/home/u/proj/.claude/worktrees/feature-a"},
		{Path: "/home/u/proj/.claude/worktrees/feature-b"},
	}

	tests := []struct {
		prefix string
		want   []string
	}{
		{"", []string{"proj", "feature-a", "feature-b"}},
		{"feat", []string{"feature-a", "feature-b"}},
		{"feature-a", []string{"feature-a"}},
		{"none", nil},
	}
	for _, tt := range tests {
		if got := worktreeNames(wts, tt.prefix); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("worktreeNames(_, %q) = %v, want %v", tt.prefix, got, tt.want)
		}
	}
}
