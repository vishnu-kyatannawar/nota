package index

import (
	"path/filepath"
	"testing"

	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

func newIndex(t *testing.T) (*Index, *vault.Vault) {
	t.Helper()
	dir := t.TempDir()
	v, err := vault.Open(filepath.Join(dir, "Notes"))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Open(filepath.Join(dir, "index.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	return idx, v
}

func seed(t *testing.T, v *vault.Vault) {
	t.Helper()
	notes := map[string]string{
		"Workplans/2026-09-01.md": `---
type: workplan
date: 2026-09-01
hours: "09:00"
daytype: work
---

- [x] Fix the auth bug #rv-api [01:20] <!--n id:A1 t:09:34 done:16:11-->
- [ ] Review PR 412 #rv-portal <!--n id:A2 t:09:40-->
`,
		"Workplans/2026-09-02.md": `---
type: workplan
date: 2026-09-02
hours: "07:30"
daytype: work
---

- [ ] Review PR 412 #rv-portal <!--n id:A2 t:09:40 from:2026-09-01 carried:1-->
- [ ] Chase the invoice #billing <!--n id:A3 t:14:02-->
`,
		"Workplans/2026-09-05.md": `---
type: workplan
date: 2026-09-05
hours: "00:00"
daytype: leave
---
`,
		"Projects/rv-license/api.md": `---
labels: [reference, rv-api]
---

# Auth notes

The middleware rejects a token when exp is in the past.
`,
	}
	for path, content := range notes {
		if err := v.WriteRaw(path, content); err != nil {
			t.Fatal(err)
		}
	}
}

// The index is a cache. Deleting it and rebuilding must produce exactly what
// incremental updates produced, or the two paths have drifted and the cheap one
// is quietly wrong.
func TestRebuildMatchesIncrementalUpdates(t *testing.T) {
	incremental, v := newIndex(t)
	seed(t, v)

	paths, err := v.ListNotes()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		note, err := v.ReadNote(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := incremental.Update(p, note); err != nil {
			t.Fatalf("Update(%s): %v", p, err)
		}
	}

	rebuilt, _ := newIndex(t)
	if err := rebuilt.Rebuild(v); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	a, err := incremental.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	b, err := rebuilt.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("incremental and rebuilt indexes differ.\n--- incremental ---\n%s\n--- rebuilt ---\n%s", a, b)
	}
}

func TestRebuildIsIdempotent(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)

	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}
	first, err := idx.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}
	second, err := idx.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("rebuilding twice produced a different index")
	}
}

func TestUpdateReplacesRatherThanDuplicates(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := idx.Update("Workplans/2026-09-02.md", note); err != nil {
			t.Fatal(err)
		}
	}

	items, err := idx.ItemsIn("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items after repeated updates, want 2", len(items))
	}
}

func TestRemoveDropsTheNoteAndItsRows(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	if err := idx.Remove("Workplans/2026-09-01.md"); err != nil {
		t.Fatal(err)
	}

	items, err := idx.ItemsIn("Workplans/2026-09-01.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("items survived the note being removed: %+v", items)
	}
	hits, err := idx.Search("auth")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.Path == "Workplans/2026-09-01.md" {
			t.Error("removed note still appears in search results")
		}
	}
}

func TestSearchFindsNoteContent(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	hits, err := idx.Search("middleware")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Path != "Projects/rv-license/api.md" {
		t.Errorf("Search(middleware) = %+v, want the api note", hits)
	}
}

func TestSearchWithNoMatchesReturnsEmpty(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	hits, err := idx.Search("nothingmatchesthis")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("got %d hits, want none", len(hits))
	}
}

// Search text comes straight from the user, so FTS5 operators in it must not
// blow up the query.
func TestSearchToleratesQuerySyntax(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{`"`, `auth OR`, `*`, `NOT`, `a AND`, `()`, `-`, ``} {
		if _, err := idx.Search(q); err != nil {
			t.Errorf("Search(%q) returned an error: %v", q, err)
		}
	}
}

func TestLabelsAreCountedAcrossNotesAndItems(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	labels, err := idx.Labels()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, l := range labels {
		counts[l.Name] = l.Count
	}

	if counts["rv-portal"] != 2 {
		t.Errorf("rv-portal count = %d, want 2", counts["rv-portal"])
	}
	if counts["billing"] != 1 {
		t.Errorf("billing count = %d, want 1", counts["billing"])
	}
	// rv-api appears on an item in one note and in another note's frontmatter.
	if counts["rv-api"] != 2 {
		t.Errorf("rv-api count = %d, want 2", counts["rv-api"])
	}
}

func TestNotesByLabel(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	paths, err := idx.NotesByLabel("billing")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "Workplans/2026-09-02.md" {
		t.Errorf("NotesByLabel(billing) = %v", paths)
	}
}

// Hours are one field per note precisely so a range total is a single query.
func TestHoursBetweenSumsTheRange(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		from, to string
		want     int
	}{
		{"both work days", "2026-09-01", "2026-09-02", 9*60 + 7*60 + 30},
		{"single day", "2026-09-01", "2026-09-01", 9 * 60},
		{"range including a leave day at zero", "2026-09-01", "2026-09-05", 9*60 + 7*60 + 30},
		{"range with nothing in it", "2026-10-01", "2026-10-31", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idx.MinutesBetween(tt.from, tt.to)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("MinutesBetween(%s, %s) = %d, want %d", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestWorkplanDatesAreListedNewestFirst(t *testing.T) {
	idx, v := newIndex(t)
	seed(t, v)
	if err := idx.Rebuild(v); err != nil {
		t.Fatal(err)
	}

	days, err := idx.Workplans()
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Fatalf("got %d workplans, want 3", len(days))
	}
	if days[0].Date != "2026-09-05" || days[2].Date != "2026-09-01" {
		t.Errorf("workplans are not newest first: %+v", days)
	}
	if days[0].DayType != "leave" {
		t.Errorf("day type = %q, want leave", days[0].DayType)
	}
	if days[1].Minutes != 7*60+30 {
		t.Errorf("minutes = %d, want 450", days[1].Minutes)
	}
}

func TestOpenOnAMissingDatabaseCreatesIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "index.db")

	idx, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if _, err := idx.Labels(); err != nil {
		t.Errorf("fresh index is not queryable: %v", err)
	}
}

func TestUpdateHandlesANoteWithNoItems(t *testing.T) {
	idx, _ := newIndex(t)
	note := &mdnote.Note{Type: "workplan", Date: "2026-09-09", Hours: "00:00", DayType: "weekend"}
	if err := idx.Update("Workplans/2026-09-09.md", note); err != nil {
		t.Fatalf("Update: %v", err)
	}
	items, err := idx.ItemsIn("Workplans/2026-09-09.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want none", len(items))
	}
}
