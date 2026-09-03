package services

import (
	"path/filepath"
	"strings"
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

func TestSaveBodyLeavesItemLinesByteIdentical(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	src := "---\ntype: workplan\ndate: 2026-09-02\nhours: \"03:00\"\ndaytype: work\n---\n\n" +
		"- [ ] One #x <!--n id:A1 t:09:00 from:2026-09-01 carried:1-->\n" +
		"      body line\n\n" +
		"  - [x] Two <!--n id:A2 t:09:01 done:10:00-->\n"
	if err := core.vault.WriteRaw("Workplans/2026-09-02.md", src); err != nil {
		t.Fatal(err)
	}

	if err := notes.SaveBody("Workplans/2026-09-02.md", "## Meeting\n\n**bold** and a list:\n\n- a\n- b"); err != nil {
		t.Fatalf("SaveBody: %v", err)
	}

	got, err := core.vault.ReadRaw("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, src) {
		t.Errorf("frontmatter or item lines changed.\nbefore:\n%s\nafter:\n%s", src, got)
	}
	if !strings.HasSuffix(got, "\n\n## Meeting\n\n**bold** and a list:\n\n- a\n- b\n") {
		t.Errorf("body not written as expected:\n%s", got)
	}

	// Clearing the body removes the section entirely rather than leaving a stub.
	if err := notes.SaveBody("Workplans/2026-09-02.md", ""); err != nil {
		t.Fatal(err)
	}
	got, _ = core.vault.ReadRaw("Workplans/2026-09-02.md")
	if got != src {
		t.Errorf("clearing the body did not restore the original:\n%s", got)
	}
}

func TestSetLayoutValidatesAndRefusesWorkplans(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	if _, err := notes.Create("Projects/list.md"); err != nil {
		t.Fatal(err)
	}
	if err := notes.SetLayout("Projects/list.md", "items"); err != nil {
		t.Fatalf("SetLayout: %v", err)
	}
	n, _ := notes.Get("Projects/list.md")
	if n.Layout != "items" {
		t.Errorf("Layout = %q", n.Layout)
	}
	if err := notes.SetLayout("Projects/list.md", "sideways"); err == nil {
		t.Error("unknown layout accepted")
	}

	if err := core.vault.WriteRaw("Workplans/2026-09-02.md", "---\ntype: workplan\ndate: 2026-09-02\n---\n"); err != nil {
		t.Fatal(err)
	}
	if err := notes.SetLayout("Workplans/2026-09-02.md", "notes"); err == nil {
		t.Error("a workplan accepted a layout")
	}
}

func TestSetLabelsCleansAndDeduplicates(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	if _, err := notes.Create("n.md"); err != nil {
		t.Fatal(err)
	}
	if err := notes.SetLabels("n.md", []string{" #work ", "work", "", "two words", "ok"}); err != nil {
		t.Fatal(err)
	}
	n, _ := notes.Get("n.md")
	want := []string{"work", "two-words", "ok"}
	if strings.Join(n.Labels, ",") != strings.Join(want, ",") {
		t.Errorf("Labels = %v, want %v", n.Labels, want)
	}
}

func TestMoveItemsToTodaysWorkplan(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	plans := NewWorkplanService(core)

	src := "---\nlayout: items\n---\n\n" +
		"- [ ] Stay <!--n id:S t:08:00-->\n" +
		"- [ ] Go #x <!--n id:P t:08:01-->\n" +
		"      parent notes\n\n" +
		"  - [ ] Go child <!--n id:C t:08:02-->\n"
	if err := core.vault.WriteRaw("Lists/backlog.md", src); err != nil {
		t.Fatal(err)
	}

	dest, err := plans.MoveItems("Lists/backlog.md", []string{"P"}, "")
	if err != nil {
		t.Fatalf("MoveItems: %v", err)
	}
	if dest == "" {
		t.Fatal("no destination returned")
	}

	from, _ := notes.Get("Lists/backlog.md")
	if len(from.Items) != 1 || from.Items[0].ID != "S" {
		t.Errorf("source after move: %+v", from.Items)
	}
	to, _ := notes.Get(dest)
	last := to.Items[len(to.Items)-2:]
	if last[0].ID != "P" || last[1].ID != "C" || last[1].Depth != 1 {
		t.Errorf("destination tail: %+v", last)
	}
	if last[0].CreatedAt != "08:01" || len(last[0].Body) == 0 {
		t.Errorf("moved item lost creation time or body: %+v", last[0])
	}

	// Moving into the same note is refused: it would silently reorder.
	if _, err := plans.MoveItems(dest, []string{"P"}, dest); err == nil {
		t.Error("move into the same note was allowed")
	}
}

func TestTrashThroughTheService(t *testing.T) {
	core := newTestCore(t)
	notes := NewNotesService(core)
	if err := core.vault.WriteRaw("Projects/api.md", "- [ ] Find me #trashed <!--n id:T1 t:09:00-->\n"); err != nil {
		t.Fatal(err)
	}
	if err := core.index.Rebuild(core.vault); err != nil {
		t.Fatal(err)
	}

	if err := notes.Delete("Projects/api.md"); err != nil {
		t.Fatal(err)
	}
	if hits, _ := core.index.Search("Find"); len(hits) != 0 {
		t.Error("deleted note still indexed")
	}
	entries, err := notes.ListTrash()
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListTrash = %+v, %v", entries, err)
	}

	restored, err := notes.Restore(entries[0].ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored != "Projects/api.md" {
		t.Errorf("restored to %q", restored)
	}
	if hits, _ := core.index.Search("Find"); len(hits) != 1 {
		t.Error("restored note not re-indexed")
	}

	if err := notes.Delete("Projects/api.md"); err != nil {
		t.Fatal(err)
	}
	if err := notes.EmptyTrash(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := notes.ListTrash(); len(entries) != 0 {
		t.Errorf("EmptyTrash left %d entries", len(entries))
	}
}
