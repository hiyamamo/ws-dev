package cmd

import (
	"path/filepath"
	"testing"
)

func TestResolveMcpLogDir(t *testing.T) {
	const wtA = "/repo/.claude/worktrees/a"
	const wtB = "/repo/.claude/worktrees/b"

	tests := []struct {
		name     string
		base     string
		flag     string
		env      string
		expected string
	}{
		{
			name:     "default is base/log",
			base:     wtA,
			expected: filepath.Join(wtA, "log"),
		},
		{
			name:     "relative env joined onto base",
			base:     wtA,
			env:      "tmp/logs",
			expected: filepath.Join(wtA, "tmp/logs"),
		},
		{
			name:     "absolute env inside base is honored",
			base:     wtA,
			env:      filepath.Join(wtA, "log"),
			expected: filepath.Join(wtA, "log"),
		},
		{
			name:     "foreign absolute env from another worktree is ignored",
			base:     wtB,
			env:      filepath.Join(wtA, "log"),
			expected: filepath.Join(wtB, "log"),
		},
		{
			name:     "flag wins over env",
			base:     wtA,
			flag:     "custom",
			env:      filepath.Join(wtB, "log"),
			expected: filepath.Join(wtA, "custom"),
		},
		{
			name:     "absolute flag honored even outside base",
			base:     wtA,
			flag:     "/var/log/ws",
			expected: "/var/log/ws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMcpLogDir(tt.base, tt.flag, tt.env)
			if got != tt.expected {
				t.Errorf("resolveMcpLogDir(%q, %q, %q) = %q, want %q",
					tt.base, tt.flag, tt.env, got, tt.expected)
			}
		})
	}
}
