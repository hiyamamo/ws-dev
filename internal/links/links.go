package links

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Link creates symlinks under repoDir for each entry declared in entries.
// entries are paths relative to linksDir and must exist there.
// Intermediate directories in the target path are created as needed.
//
// A trailing "/" puts an entry in contents mode: each immediate child of
// linksDir/<entry> is symlinked into repoDir/<entry>, leaving the parent
// directory intact so tracked files like .gitkeep survive.
func Link(linksDir, repoDir string, entries []string) error {
	for _, rel := range entries {
		clean, contents := contentsMode(rel)
		src := filepath.Join(linksDir, clean)
		if _, err := os.Lstat(src); err != nil {
			return fmt.Errorf("missing link source %s: %w", src, err)
		}
		target := filepath.Join(repoDir, clean)

		if contents {
			if err := linkContents(src, target); err != nil {
				return fmt.Errorf("link contents %s -> %s: %w", target, src, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeSymlink(src, target, false); err != nil {
			return fmt.Errorf("link %s -> %s: %w", target, src, err)
		}
	}
	return nil
}

// Unlink removes symlinks created by Link. linksDir is consulted for
// contents-mode entries to know which children were linked.
// Non-symlink entries are left alone to avoid clobbering real data.
func Unlink(linksDir, repoDir string, entries []string) error {
	for _, rel := range entries {
		clean, contents := contentsMode(rel)

		if contents {
			if err := unlinkContents(filepath.Join(linksDir, clean), filepath.Join(repoDir, clean)); err != nil {
				return err
			}
			continue
		}

		target := filepath.Join(repoDir, clean)
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

func contentsMode(rel string) (string, bool) {
	if strings.HasSuffix(rel, "/") {
		return strings.TrimRight(rel, "/"), true
	}
	return rel, false
}

func linkContents(srcDir, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := writeSymlink(filepath.Join(srcDir, e.Name()), filepath.Join(targetDir, e.Name()), true); err != nil {
			return err
		}
	}
	return nil
}

func unlinkContents(srcDir, targetDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "skip %s (links source missing)\n", srcDir)
			return nil
		}
		return err
	}
	for _, e := range entries {
		target := filepath.Join(targetDir, e.Name())
		info, err := os.Lstat(target)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if err := os.Remove(target); err != nil {
			return err
		}
	}
	return nil
}

// writeSymlink creates a relative symlink at target pointing to src. Stale
// symlinks at target are replaced. If preserveReal is true, a real file or
// directory at target is left in place with a stderr warning; otherwise it
// is removed first.
func writeSymlink(src, target string, preserveReal bool) error {
	info, err := os.Lstat(target)
	if err == nil {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			if err := os.Remove(target); err != nil {
				return err
			}
		case preserveReal:
			fmt.Fprintf(os.Stderr, "skip %s (exists as real file)\n", target)
			return nil
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
