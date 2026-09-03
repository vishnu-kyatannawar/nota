// Package export writes and reads the backup bundle.
//
// Because the markdown files are the source of truth, a backup is simply a copy
// of the vault and a restore is simply putting it back. There is no database
// dump to reconcile and no partial state to repair, which is what makes the
// round trip complete by construction rather than by careful bookkeeping.
//
// The index is deliberately left out: it is derived from the notes and rebuilt
// on the next launch, so shipping it would only add bulk and risk restoring a
// stale cache over fresh notes.
package export

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

// IndexFileName is the derived database, excluded from every bundle.
const IndexFileName = "index.db"

// ErrUnsafePath is returned when a bundle entry would write outside the vault.
var ErrUnsafePath = errors.New("bundle entry points outside the vault")

// BundleName is the conventional filename for a bundle taken on a given date.
func BundleName(date string) string {
	return "nota-export-" + date + ".zip"
}

// Export writes the whole vault to a zip at dest.
func Export(v *vault.Vault, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating export directory: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating bundle: %w", err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	root := v.Root()

	walkErr := filepath.WalkDir(root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == IndexFileName {
			return nil
		}

		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)

		src, err := os.Open(abs)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		defer func() { _ = src.Close() }()

		entry, err := w.Create(name)
		if err != nil {
			return fmt.Errorf("adding %s to the bundle: %w", name, err)
		}
		if _, err := io.Copy(entry, src); err != nil {
			return fmt.Errorf("writing %s to the bundle: %w", name, err)
		}
		return nil
	})
	if walkErr != nil {
		_ = w.Close()
		return walkErr
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finishing the bundle: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing the bundle: %w", err)
	}
	return nil
}

// Restore extracts a bundle into a vault, replacing files it contains and
// leaving anything else alone.
//
// A bundle is a file from outside the application, so every entry name is
// checked before anything is written: a zip entry naming a path above the
// destination would otherwise write wherever it liked on the user's machine.
func Restore(bundle string, v *vault.Vault) error {
	r, err := zip.OpenReader(bundle)
	if err != nil {
		return fmt.Errorf("opening bundle: %w", err)
	}
	defer func() { _ = r.Close() }()

	root := v.Root()

	// Validate every entry up front, so a malicious bundle is rejected whole
	// rather than half-extracted.
	for _, entry := range r.File {
		if _, err := safeJoin(root, entry.Name); err != nil {
			return err
		}
	}

	for _, entry := range r.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		dest, err := safeJoin(root, entry.Name)
		if err != nil {
			return err
		}
		if err := extract(entry, dest); err != nil {
			return err
		}
	}
	return nil
}

func extract(entry *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating folder for %s: %w", entry.Name, err)
	}

	src, err := entry.Open()
	if err != nil {
		return fmt.Errorf("reading %s from the bundle: %w", entry.Name, err)
	}
	defer func() { _ = src.Close() }()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("writing %s: %w", entry.Name, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("writing %s: %w", entry.Name, err)
	}
	return out.Close()
}

// safeJoin resolves a bundle entry name against the vault root, refusing
// absolute paths and anything that climbs out through "..".
func safeJoin(root, name string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", fmt.Errorf("%w: %s", ErrUnsafePath, name)
	}
	clean := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %s", ErrUnsafePath, name)
	}

	dest := filepath.Join(root, filepath.FromSlash(clean))
	if dest != root && !strings.HasPrefix(dest, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrUnsafePath, name)
	}
	return dest, nil
}
