package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// oldSuffix marks the outgoing binary on Windows, where a running executable
// cannot be overwritten but can be renamed out of the way.
const oldSuffix = ".old"

// Target is the path this process would replace: its own executable, with any
// symlinks resolved so an update lands on the real file rather than the link.
func Target() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding this executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		// A path that cannot be resolved is still worth reporting as-is; the
		// writability check is what actually decides whether we may replace it.
		return exe, nil //nolint:nilerr // the unresolved path is a usable answer
	}
	return resolved, nil
}

// CanReplace reports whether this process may put a new binary at target.
//
// Replacement works by renaming into place, so what must be writable is the
// directory, not the file. A package-managed /usr/bin/nota fails here and the
// caller offers the release page instead of fighting the package manager.
func CanReplace(target string) bool {
	dir := filepath.Dir(target)
	probe, err := os.CreateTemp(dir, ".nota-write-")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return true
}

// Apply puts the staged binary at target.
//
// On Linux the rename replaces the directory entry while the running process
// keeps its own inode, so the app carries on working until it is restarted.
// On Windows the running image cannot be written over, but it can be renamed,
// so the outgoing binary is moved aside and swept up at the next launch.
func Apply(staged, target string) error {
	if err := os.Chmod(staged, 0o755); err != nil {
		return fmt.Errorf("making %s executable: %w", staged, err)
	}
	// Stage beside the target: a rename cannot cross filesystems, and /tmp is
	// very often a different one from ~/.local/bin.
	next := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".new")
	if err := copyFile(staged, next); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		old := target + oldSuffix
		_ = os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			_ = os.Remove(next)
			return fmt.Errorf("moving the running %s aside: %w", filepath.Base(target), err)
		}
		if err := os.Rename(next, target); err != nil {
			// Put the original back rather than leaving nothing installed.
			_ = os.Rename(old, target)
			_ = os.Remove(next)
			return fmt.Errorf("installing %s: %w", target, err)
		}
		return nil
	}

	if err := os.Rename(next, target); err != nil {
		_ = os.Remove(next)
		return fmt.Errorf("installing %s: %w", target, err)
	}
	return nil
}

// CleanupOld removes the binary left aside by a previous Windows update. It is
// called at launch and never reports failure: the file is harmless if it stays.
func CleanupOld(target string) {
	_ = os.Remove(target + oldSuffix)
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("opening %s: %w", from, err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("creating %s: %w", to, err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("writing %s: %w", to, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", to, err)
	}
	return nil
}
