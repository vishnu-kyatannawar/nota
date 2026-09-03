// Package vault is the only code that touches the notes directory.
//
// The vault is a plain tree of markdown files that the user also opens in other
// editors, so this package treats the filesystem as the source of truth: it owns
// reads, writes, moves and deletes, and it watches for changes made outside the
// application. Every path crossing this boundary is vault-relative and validated,
// because a path that escapes the root would read or write arbitrary files.
package vault

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"

	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
)

// NoteExt is the only file extension the vault treats as a note.
const NoteExt = ".md"

// AppDirName is the vault-internal folder holding settings, templates and the
// index. It is skipped when walking the tree.
const AppDirName = ".nota"

// ErrOutsideVault is returned for any path that does not resolve inside the root.
var ErrOutsideVault = errors.New("path is outside the vault")

// ErrExists is returned when a move would overwrite something already there.
var ErrExists = errors.New("destination already exists")

// Vault is an open notes directory.
type Vault struct {
	root string
}

// Node is one entry in the folder tree.
type Node struct {
	// Name is the display name: a folder's directory name, or a note's filename
	// without the .md extension.
	Name string `json:"name"`
	// Path is the vault-relative path, always with forward slashes.
	Path     string  `json:"path"`
	IsFolder bool    `json:"isFolder"`
	Children []*Node `json:"children,omitempty"`
}

// Change describes a note that was modified outside the application.
type Change struct {
	Path string `json:"path"`
	Op   string `json:"op"`
}

// Open prepares the directory at root as a vault, creating it if needed.
func Open(root string) (*Vault, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving vault root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("creating vault root: %w", err)
	}
	return &Vault{root: abs}, nil
}

// Root is the absolute path of the vault directory.
func (v *Vault) Root() string { return v.root }

// resolve turns a vault-relative path into an absolute one, refusing anything
// that would land outside the vault. Absolute inputs, empty inputs and any path
// climbing through ".." are rejected rather than normalised, so a caller cannot
// accidentally address the wider filesystem.
func (v *Vault) resolve(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("%w: empty path", ErrOutsideVault)
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("%w: %s", ErrOutsideVault, rel)
	}

	clean := path.Clean(strings.ReplaceAll(rel, string(filepath.Separator), "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %s", ErrOutsideVault, rel)
	}

	abs := filepath.Join(v.root, filepath.FromSlash(clean))

	// Join already removed any interior "..", but compare anyway so a symlinked
	// or unusual root cannot widen what is reachable.
	if abs != v.root && !strings.HasPrefix(abs, v.root+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideVault, rel)
	}
	return abs, nil
}

// ReadNote parses the note at a vault-relative path.
func (v *Vault) ReadNote(rel string) (*mdnote.Note, error) {
	abs, err := v.resolve(rel)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("reading note %s: %w", rel, err)
	}
	note, err := mdnote.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing note %s: %w", rel, err)
	}
	return note, nil
}

// ReadRaw returns the note's bytes without parsing, for the raw markdown editor.
func (v *Vault) ReadRaw(rel string) (string, error) {
	abs, err := v.resolve(rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("reading note %s: %w", rel, err)
	}
	return string(data), nil
}

// WriteNote serialises a note and replaces the file at a vault-relative path.
func (v *Vault) WriteNote(rel string, note *mdnote.Note) error {
	return v.WriteRaw(rel, mdnote.Serialize(note))
}

// WriteRaw replaces a note's contents. The write goes to a temporary file in the
// same directory and is then renamed, so a crash mid-write cannot truncate a note
// the user already has.
func (v *Vault) WriteRaw(rel, content string) error {
	abs, err := v.resolve(rel)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating folder for %s: %w", rel, err)
	}

	tmp, err := os.CreateTemp(dir, ".note-*")
	if err != nil {
		return fmt.Errorf("creating temporary file for %s: %w", rel, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", rel, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", rel, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", rel, err)
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return fmt.Errorf("installing %s: %w", rel, err)
	}
	return nil
}

// CreateFolder creates a folder and any missing parents.
func (v *Vault) CreateFolder(rel string) error {
	abs, err := v.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf("creating folder %s: %w", rel, err)
	}
	return nil
}

// Rename moves a note or folder. It refuses to overwrite an existing path, since
// silently replacing a note the user cannot see would lose their work.
func (v *Vault) Rename(from, to string) error {
	src, err := v.resolve(from)
	if err != nil {
		return err
	}
	dst, err := v.resolve(to)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%w: %s", ErrExists, to)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", to, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating folder for %s: %w", to, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", from, to, err)
	}
	return nil
}

// Delete removes a note, or a folder and everything under it.
func (v *Vault) Delete(rel string) error {
	abs, err := v.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(abs); err != nil {
		return fmt.Errorf("deleting %s: %w", rel, err)
	}
	return nil
}

// Tree walks the vault and returns its folders and notes. Folders sort before
// notes and each group sorts by name, which is the order the sidebar shows.
func (v *Vault) Tree() (*Node, error) {
	root := &Node{Name: filepath.Base(v.root), Path: "", IsFolder: true}
	children, err := v.readDir(v.root, "")
	if err != nil {
		return nil, err
	}
	root.Children = children
	return root, nil
}

func (v *Vault) readDir(abs, rel string) ([]*Node, error) {
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("reading folder %s: %w", abs, err)
	}

	var folders, notes []*Node
	for _, e := range entries {
		name := e.Name()
		// A vault lives alongside other tools' metadata: skip our own folder and
		// anything hidden, rather than showing .git and .obsidian in the sidebar.
		if strings.HasPrefix(name, ".") {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}

		if e.IsDir() {
			sub, err := v.readDir(filepath.Join(abs, name), childRel)
			if err != nil {
				return nil, err
			}
			folders = append(folders, &Node{Name: name, Path: childRel, IsFolder: true, Children: sub})
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), NoteExt) {
			continue
		}
		notes = append(notes, &Node{Name: strings.TrimSuffix(name, filepath.Ext(name)), Path: childRel})
	}

	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	sort.Slice(notes, func(i, j int) bool { return notes[i].Name < notes[j].Name })
	return append(folders, notes...), nil
}

// ListNotes returns every note path in the vault, forward-slashed and sorted.
func (v *Vault) ListNotes() ([]string, error) {
	var out []string
	err := filepath.WalkDir(v.root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if abs != v.root && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), NoteExt) {
			return nil
		}
		rel, err := filepath.Rel(v.root, abs)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing notes: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

// Watch reports notes changed outside the application until ctx is cancelled,
// at which point the returned channel is closed. Editors often write through a
// temporary file, so create, write and rename all surface as a change.
func (v *Vault) Watch(ctx context.Context) (<-chan Change, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	if err := v.addWatchDirs(w); err != nil {
		_ = w.Close()
		return nil, err
	}

	out := make(chan Change, 64)
	go func() {
		defer close(out)
		defer func() { _ = w.Close() }()

		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				_ = err // a watch error must not take the application down
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				// A new folder needs its own watch; fsnotify is not recursive.
				if ev.Has(fsnotify.Create) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						_ = w.Add(ev.Name)
						continue
					}
				}
				change, ok := v.changeFor(ev)
				if !ok {
					continue
				}
				select {
				case out <- change:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

func (v *Vault) addWatchDirs(w *fsnotify.Watcher) error {
	return filepath.WalkDir(v.root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if abs != v.root && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		if err := w.Add(abs); err != nil {
			return fmt.Errorf("watching %s: %w", abs, err)
		}
		return nil
	})
}

// changeFor filters a filesystem event down to a note the user would care about,
// discarding our own atomic-write temporaries and non-markdown files.
func (v *Vault) changeFor(ev fsnotify.Event) (Change, bool) {
	name := filepath.Base(ev.Name)
	if strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), NoteExt) {
		return Change{}, false
	}
	rel, err := filepath.Rel(v.root, ev.Name)
	if err != nil || strings.HasPrefix(rel, "..") {
		return Change{}, false
	}

	switch {
	case ev.Has(fsnotify.Remove), ev.Has(fsnotify.Rename):
		return Change{Path: filepath.ToSlash(rel), Op: "removed"}, true
	case ev.Has(fsnotify.Create), ev.Has(fsnotify.Write):
		return Change{Path: filepath.ToSlash(rel), Op: "changed"}, true
	default:
		return Change{}, false
	}
}
