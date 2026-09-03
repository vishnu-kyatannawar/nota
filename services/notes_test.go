package services

import (
	"path/filepath"
	"testing"

	"github.com/vishnu-kyatannawar/nota/internal/config"
)

func newTestCore(t *testing.T) *Core {
	t.Helper()
	s := config.DefaultSettings()
	s.VaultPath = filepath.Join(t.TempDir(), "Notes")
	core, err := NewCore("test", s)
	if err != nil {
		t.Fatalf("NewCore: %v", err)
	}
	t.Cleanup(func() { _ = core.Close() })
	return core
}

// The workplan folder is where every day's note is created; renaming or
// deleting it from the sidebar would silently break rollover.
func TestReservedWorkplanFolderCannotBeRenamedOrDeleted(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	if err := notes.CreateFolder("Workplans"); err != nil {
		t.Fatal(err)
	}

	if err := notes.Rename("Workplans", "Days"); err == nil {
		t.Error("renaming the workplan folder was allowed")
	}
	if err := notes.Delete("Workplans"); err == nil {
		t.Error("deleting the workplan folder was allowed")
	}
	// Notes inside it are ordinary and stay editable.
	if _, err := notes.Create("Workplans/2026-09-02.md"); err != nil {
		t.Fatalf("Create inside the workplan folder: %v", err)
	}
	if err := notes.Delete("Workplans/2026-09-02.md"); err != nil {
		t.Errorf("deleting a note inside the workplan folder: %v", err)
	}
}

func TestCreateNoteReturnsAnUnusedPath(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)

	first, err := notes.Create("Projects/Untitled.md")
	if err != nil {
		t.Fatal(err)
	}
	second, err := notes.Create("Projects/Untitled.md")
	if err != nil {
		t.Fatal(err)
	}
	third, err := notes.Create("Projects/Untitled.md")
	if err != nil {
		t.Fatal(err)
	}

	if first != "Projects/Untitled.md" || second != "Projects/Untitled 2.md" || third != "Projects/Untitled 3.md" {
		t.Errorf("Create did not pick unused names: %q, %q, %q", first, second, third)
	}
}

func TestSaveItemsMintsIdsAndKeepsMetadata(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	plans := NewWorkplanService(core)

	if err := core.vault.WriteRaw("Workplans/2026-09-02.md",
		"---\ntype: workplan\ndate: 2026-09-02\nhours: \"00:00\"\ndaytype: work\n---\n\n"+
			"- [ ] Carried <!--n id:A1 t:09:40 from:2026-09-01 carried:2-->\n"); err != nil {
		t.Fatal(err)
	}

	saved, err := plans.SaveItems("Workplans/2026-09-02.md", []ItemInput{
		{ID: "A1", Text: "Carried, edited", Done: true},
		{Text: "Typed just now"},
		{Text: "Pasted child", Depth: 1},
	})
	if err != nil {
		t.Fatalf("SaveItems: %v", err)
	}
	if len(saved) != 3 {
		t.Fatalf("got %d items, want 3", len(saved))
	}
	if saved[0].Carried != 2 || saved[0].From != "2026-09-01" || saved[0].CreatedAt != "09:40" {
		t.Errorf("existing item lost metadata: %+v", saved[0])
	}
	if saved[0].DoneAt == "" {
		t.Error("ticking an item did not stamp DoneAt")
	}
	if saved[1].ID == "" || saved[2].ID == "" || saved[1].ID == saved[2].ID {
		t.Errorf("new items need distinct ids: %q, %q", saved[1].ID, saved[2].ID)
	}
	if saved[1].CreatedAt == "" {
		t.Error("new item was not stamped with a creation time")
	}

	got, err := notes.Get("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 3 || got.Items[2].Depth != 1 {
		t.Errorf("saved note does not match: %+v", got.Items)
	}
}
