package links

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Link creates symlinks under repoDir for each entry declared in entries.
// entries are paths relative to linksDir and must exist there.
// Intermediate directories in the target path are created as needed.
func Link(linksDir, repoDir string, entries []string) error {
	for _, rel := range entries {
		src := filepath.Join(linksDir, rel)
		if _, err := os.Lstat(src); err != nil {
			return fmt.Errorf("missing link source %s: %w", src, err)
		}
		target := filepath.Join(repoDir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := replaceWithSymlink(src, target); err != nil {
			return fmt.Errorf("link %s -> %s: %w", target, src, err)
		}
	}
	return nil
}

// Unlink removes each entry from repoDir if it is a symlink.
// Non-symlink entries are left alone to avoid clobbering real data.
func Unlink(repoDir string, entries []string) error {
	for _, rel := range entries {
		target := filepath.Join(repoDir, rel)
		info, err := os.Lstat(target)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			fmt.Fprintf(os.Stderr, "skip %s (not a symlink)\n", target)
			continue
		}
		if err := os.Remove(target); err != nil {
			return err
		}
	}
	return nil
}

func replaceWithSymlink(src, target string) error {
	info, err := os.Lstat(target)
	if err == nil {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			if err := os.Remove(target); err != nil {
				return err
			}
		case info.IsDir():
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		default:
			if err := os.Remove(target); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	rel, err := relativeLink(src, target)
	if err != nil {
		return err
	}
	return os.Symlink(rel, target)
}

// relativeLink returns a path from target's directory to src.
func relativeLink(src, target string) (string, error) {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Rel(filepath.Dir(absTarget), absSrc)
}
