package vault

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
)

func newVault(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return v
}

func TestOpenCreatesTheVaultDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "Notes")
	if _, err := Open(root); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Errorf("vault root was not created: %v", err)
	}
}

func TestCreateAndReadNote(t *testing.T) {
	v := newVault(t)

	note := &mdnote.Note{Type: "workplan", Date: "2026-09-02", Hours: "01:20", DayType: "work"}
	note.Items = []mdnote.Item{{Text: "Do the thing", ID: "A1", CreatedAt: "09:00"}}

	if err := v.WriteNote("Workplans/2026-09-02.md", note); err != nil {
		t.Fatalf("WriteNote: %v", err)
	}

	got, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if got.Date != "2026-09-02" {
		t.Errorf("Date = %q", got.Date)
	}
	if len(got.Items) != 1 || got.Items[0].Text != "Do the thing" {
		t.Errorf("Items = %+v", got.Items)
	}
}

func TestWriteNoteCreatesParentFolders(t *testing.T) {
	v := newVault(t)
	if err := v.WriteNote("Projects/rv-license/api.md", &mdnote.Note{}); err != nil {
		t.Fatalf("WriteNote: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), "Projects", "rv-license", "api.md")); err != nil {
		t.Errorf("note was not written: %v", err)
	}
}

// The vault is the user's data. A relative path that climbs out of it must be
// refused rather than silently reading or writing somewhere else on disk.
func TestPathsEscapingTheVaultAreRejected(t *testing.T) {
	v := newVault(t)

	escapes := []string{
		"../outside.md",
		"Projects/../../outside.md",
		"/etc/passwd",
		"Projects/../../../tmp/evil.md",
		"",
		".",
		"..",
	}
	for _, p := range escapes {
		t.Run(p, func(t *testing.T) {
			if err := v.WriteNote(p, &mdnote.Note{}); err == nil {
				t.Errorf("WriteNote(%q) was allowed", p)
			}
			if _, err := v.ReadNote(p); err == nil {
				t.Errorf("ReadNote(%q) was allowed", p)
			}
			if err := v.CreateFolder(p); err == nil {
				t.Errorf("CreateFolder(%q) was allowed", p)
			}
		})
	}
}

func TestTreeReportsNestedFoldersAndNotes(t *testing.T) {
	v := newVault(t)
	for _, p := range []string{
		"Workplans/2026-09-01.md",
		"Workplans/2026-09-02.md",
		"Projects/rv-license/api.md",
		"Projects/notes.md",
	} {
		if err := v.WriteNote(p, &mdnote.Note{}); err != nil {
			t.Fatal(err)
		}
	}

	tree, err := v.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}

	// Folders sort before notes, and each group sorts by name.
	if len(tree.Children) != 2 {
		t.Fatalf("root has %d children, want 2", len(tree.Children))
	}
	if tree.Children[0].Name != "Projects" || !tree.Children[0].IsFolder {
		t.Errorf("first child = %+v, want the Projects folder", tree.Children[0])
	}

	projects := tree.Children[0]
	if len(projects.Children) != 2 {
		t.Fatalf("Projects has %d children, want 2", len(projects.Children))
	}
	if !projects.Children[0].IsFolder || projects.Children[0].Name != "rv-license" {
		t.Errorf("folders must sort before notes, got %+v", projects.Children[0])
	}
	if projects.Children[1].Name != "notes" {
		t.Errorf("note name should drop the .md extension, got %q", projects.Children[1].Name)
	}
}

// A vault is a directory the user also opens in other tools, so the tree must
// ignore the app's own folder and anything hidden.
func TestTreeSkipsHiddenAndAppDirectories(t *testing.T) {
	v := newVault(t)
	if err := v.WriteNote("Real.md", &mdnote.Note{}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{".nota", ".git", ".obsidian"} {
		if err := os.MkdirAll(filepath.Join(v.Root(), dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(v.Root(), dir, "x.md"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tree, err := v.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Children) != 1 || tree.Children[0].Name != "Real" {
		t.Errorf("tree = %+v, want only the Real note", tree.Children)
	}
}

func TestTreeIgnoresNonMarkdownFiles(t *testing.T) {
	v := newVault(t)
	if err := os.WriteFile(filepath.Join(v.Root(), "image.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("Real.md", &mdnote.Note{}); err != nil {
		t.Fatal(err)
	}

	tree, err := v.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Children) != 1 || tree.Children[0].Name != "Real" {
		t.Errorf("tree = %+v, want only the markdown note", tree.Children)
	}
}

func TestCreateFolderAndNestedFolder(t *testing.T) {
	v := newVault(t)
	if err := v.CreateFolder("Reference/Books/Go"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if info, err := os.Stat(filepath.Join(v.Root(), "Reference", "Books", "Go")); err != nil || !info.IsDir() {
		t.Errorf("nested folder was not created: %v", err)
	}
}

func TestRenameNoteAndFolder(t *testing.T) {
	v := newVault(t)
	if err := v.WriteNote("Projects/old.md", &mdnote.Note{Type: "note"}); err != nil {
		t.Fatal(err)
	}

	if err := v.Rename("Projects/old.md", "Projects/new.md"); err != nil {
		t.Fatalf("Rename note: %v", err)
	}
	if _, err := v.ReadNote("Projects/new.md"); err != nil {
		t.Errorf("renamed note not readable: %v", err)
	}
	if _, err := v.ReadNote("Projects/old.md"); err == nil {
		t.Error("old path is still readable after rename")
	}

	if err := v.Rename("Projects", "Work"); err != nil {
		t.Fatalf("Rename folder: %v", err)
	}
	if _, err := v.ReadNote("Work/new.md"); err != nil {
		t.Errorf("note not readable under the renamed folder: %v", err)
	}
}

func TestRenameRefusesToOverwriteAnExistingPath(t *testing.T) {
	v := newVault(t)
	if err := v.WriteNote("a.md", &mdnote.Note{}); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("b.md", &mdnote.Note{}); err != nil {
		t.Fatal(err)
	}
	if err := v.Rename("a.md", "b.md"); err == nil {
		t.Error("rename over an existing note was allowed")
	}
}

func TestDeleteNoteAndFolder(t *testing.T) {
	v := newVault(t)
	if err := v.WriteNote("Scratch/one.md", &mdnote.Note{}); err != nil {
		t.Fatal(err)
	}

	if err := v.Delete("Scratch/one.md"); err != nil {
		t.Fatalf("Delete note: %v", err)
	}
	if _, err := v.ReadNote("Scratch/one.md"); err == nil {
		t.Error("note is still readable after delete")
	}

	if err := v.Delete("Scratch"); err != nil {
		t.Fatalf("Delete folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), "Scratch")); !os.IsNotExist(err) {
		t.Error("folder still exists after delete")
	}
}

func TestListNotesReturnsEveryMarkdownPath(t *testing.T) {
	v := newVault(t)
	for _, p := range []string{"a.md", "x/b.md", "x/y/c.md"} {
		if err := v.WriteNote(p, &mdnote.Note{}); err != nil {
			t.Fatal(err)
		}
	}

	paths, err := v.ListNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("ListNotes() = %v, want 3 entries", paths)
	}
	for _, p := range paths {
		if strings.Contains(p, string(os.PathSeparator)) && !strings.Contains(p, "/") {
			t.Errorf("path %q should use forward slashes so it is stable across platforms", p)
		}
	}
}

func TestWriteIsAtomicallyReplaced(t *testing.T) {
	v := newVault(t)
	if err := v.WriteNote("a.md", &mdnote.Note{Type: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNote("a.md", &mdnote.Note{Type: "second"}); err != nil {
		t.Fatal(err)
	}

	got, err := v.ReadNote("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "second" {
		t.Errorf("Type = %q, want the second write", got.Type)
	}

	// A crashed write must not leave temporary files lying around in the vault.
	entries, err := os.ReadDir(v.Root())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".note-") {
			t.Errorf("temporary file %q was left behind", e.Name())
		}
	}
}

func TestWatchReportsExternalChanges(t *testing.T) {
	v := newVault(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := v.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Simulate the user editing a note in another editor.
	if err := os.WriteFile(filepath.Join(v.Root(), "external.md"), []byte("- [ ] typed in vim\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-events:
		if ev.Path != "external.md" {
			t.Errorf("event path = %q, want external.md", ev.Path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no change event arrived within 5s")
	}
}

func TestWatchStopsWhenTheContextIsCancelled(t *testing.T) {
	v := newVault(t)
	ctx, cancel := context.WithCancel(context.Background())

	events, err := v.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	cancel()

	select {
	case _, open := <-events:
		if open {
			// Draining a final buffered event is fine; the channel must still close.
			select {
			case _, open := <-events:
				if open {
					t.Error("event channel did not close after cancellation")
				}
			case <-time.After(2 * time.Second):
				t.Error("event channel did not close after cancellation")
			}
		}
	case <-time.After(2 * time.Second):
		t.Error("event channel did not close after cancellation")
	}
}

func TestDeleteMovesToTrashAndRestoreBringsItBack(t *testing.T) {
	v := newVault(t)
	if err := v.WriteRaw("Projects/api.md", "# API\n\nkeep me\n"); err != nil {
		t.Fatal(err)
	}

	if err := v.Delete("Projects/api.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := v.ReadRaw("Projects/api.md"); err == nil {
		t.Fatal("note still readable after delete")
	}
	tree, _ := v.Tree()
	for _, c := range tree.Children {
		if c.Name == "Projects" {
			for _, n := range c.Children {
				if n.Name == "api" {
					t.Fatal("deleted note still in the tree")
				}
			}
		}
	}

	entries, err := v.ListTrash()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("trash has %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Path != "Projects/api.md" || e.IsFolder || e.ID == "" || e.DeletedAt.IsZero() {
		t.Errorf("trash entry = %+v", e)
	}

	restored, err := v.Restore(e.ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored != "Projects/api.md" {
		t.Errorf("restored to %q", restored)
	}
	got, err := v.ReadRaw("Projects/api.md")
	if err != nil || got != "# API\n\nkeep me\n" {
		t.Errorf("restored content = %q, err %v", got, err)
	}
	if entries, _ := v.ListTrash(); len(entries) != 0 {
		t.Errorf("trash still has %d entries after restore", len(entries))
	}
}

func TestTrashedFolderRestoresWithItsChildren(t *testing.T) {
	v := newVault(t)
	for _, p := range []string{"Work/a.md", "Work/sub/b.md"} {
		if err := v.WriteRaw(p, p+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.Delete("Work"); err != nil {
		t.Fatal(err)
	}
	entries, _ := v.ListTrash()
	if len(entries) != 1 || !entries[0].IsFolder || entries[0].Path != "Work" {
		t.Fatalf("entries = %+v", entries)
	}
	if _, err := v.Restore(entries[0].ID); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"Work/a.md", "Work/sub/b.md"} {
		if got, err := v.ReadRaw(p); err != nil || got != p+"\n" {
			t.Errorf("%s after restore = %q, %v", p, got, err)
		}
	}
}

// Restoring must never overwrite something the user made in the meantime.
func TestRestoreBesideAnOccupiedPath(t *testing.T) {
	v := newVault(t)
	if err := v.WriteRaw("note.md", "old\n"); err != nil {
		t.Fatal(err)
	}
	if err := v.Delete("note.md"); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteRaw("note.md", "new\n"); err != nil {
		t.Fatal(err)
	}
	entries, _ := v.ListTrash()
	restored, err := v.Restore(entries[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored != "note (restored).md" {
		t.Errorf("restored path = %q", restored)
	}
	if got, _ := v.ReadRaw("note.md"); got != "new\n" {
		t.Error("restore overwrote the newer note")
	}
	if got, _ := v.ReadRaw("note (restored).md"); got != "old\n" {
		t.Errorf("restored copy = %q", got)
	}
}

func TestTrashListsNewestFirstAndPurgesByAge(t *testing.T) {
	v := newVault(t)
	for _, p := range []string{"one.md", "two.md", "three.md"} {
		if err := v.WriteRaw(p, p); err != nil {
			t.Fatal(err)
		}
		if err := v.Delete(p); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := v.ListTrash()
	if len(entries) != 3 || entries[0].Path != "three.md" || entries[2].Path != "one.md" {
		t.Fatalf("not newest first: %+v", entries)
	}

	// Backdate the oldest as if it had sat there for 40 days.
	if err := v.backdateTrash(entries[2].ID, 40*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := v.PurgeTrash(30 * 24 * time.Hour); err != nil {
		t.Fatal(err)
	}
	entries, _ = v.ListTrash()
	if len(entries) != 2 {
		t.Errorf("purge left %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		if e.Path == "one.md" {
			t.Error("the 40-day-old entry survived the 30-day purge")
		}
	}
}

func TestDeleteForeverAndEmptyTrash(t *testing.T) {
	v := newVault(t)
	for _, p := range []string{"a.md", "b.md"} {
		if err := v.WriteRaw(p, p); err != nil {
			t.Fatal(err)
		}
		if err := v.Delete(p); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := v.ListTrash()
	if err := v.DeleteForever(entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if entries, _ = v.ListTrash(); len(entries) != 1 {
		t.Fatalf("DeleteForever left %d", len(entries))
	}
	if err := v.EmptyTrash(); err != nil {
		t.Fatal(err)
	}
	if entries, _ = v.ListTrash(); len(entries) != 0 {
		t.Errorf("EmptyTrash left %d", len(entries))
	}
	// Ids are directory names; one that tries to escape must be refused.
	if err := v.DeleteForever("../../etc"); err == nil {
		t.Error("DeleteForever accepted a path-like id")
	}
}

func TestTheAppDirectoryCannotBeDeleted(t *testing.T) {
	v := newVault(t)
	if err := os.MkdirAll(filepath.Join(v.Root(), AppDirName, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := v.Delete(AppDirName); err == nil {
		t.Error("deleting .nota was allowed")
	}
	if err := v.Delete(AppDirName + "/templates"); err == nil {
		t.Error("deleting inside .nota was allowed")
	}
}
