package mdnote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The format's whole promise is that a note survives a trip through the app
// unchanged. Every golden file here is parsed, serialised, and compared byte for
// byte against the original; anything that does not round-trip is a data-loss bug.
func TestGoldenFilesRoundTripByteForByte(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no golden files found in testdata/")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			note, err := Parse(string(want))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got := Serialize(note)

			if got != string(want) {
				t.Errorf("round trip changed the file.\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

// Serialisation must be idempotent even for input that is not already canonical,
// otherwise saving a note twice would keep producing new diffs.
func TestSerializeIsIdempotent(t *testing.T) {
	inputs := []string{
		"",
		"just a body\n",
		"- [ ] no frontmatter\n",
		"---\ntype: workplan\n---\n\n- [x] done\n",
		"---\ntype: workplan\n---\n- [ ] no blank line after frontmatter\n",
		"- [ ] trailing spaces   \n",
		"- [ ]   extra gap in text\n",
		"\n\n\n",
		"- [X] capital x means done\n",
	}

	for _, in := range inputs {
		t.Run(strings.ReplaceAll(in, "\n", "\\n"), func(t *testing.T) {
			first, err := Parse(in)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			once := Serialize(first)

			second, err := Parse(once)
			if err != nil {
				t.Fatalf("Parse of serialised output: %v", err)
			}
			twice := Serialize(second)

			if once != twice {
				t.Errorf("not idempotent.\nfirst:  %q\nsecond: %q", once, twice)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	in := `---
id: 01K6M2QW8ZP4
type: workplan
date: 2026-09-02
hours: "07:30"
daytype: leave
labels: [work, urgent]
---

- [ ] something
`
	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}

	if note.ID != "01K6M2QW8ZP4" {
		t.Errorf("ID = %q", note.ID)
	}
	if note.Type != "workplan" {
		t.Errorf("Type = %q", note.Type)
	}
	if note.Date != "2026-09-02" {
		t.Errorf("Date = %q", note.Date)
	}
	if note.Hours != "07:30" {
		t.Errorf("Hours = %q", note.Hours)
	}
	if note.DayType != "leave" {
		t.Errorf("DayType = %q", note.DayType)
	}
	if len(note.Labels) != 2 || note.Labels[0] != "work" || note.Labels[1] != "urgent" {
		t.Errorf("Labels = %v", note.Labels)
	}
}

func TestParseItems(t *testing.T) {
	in := `---
type: workplan
---

- [ ] Check calendar #daily <!--n id:A1 t:08:55 rec:daily-->
- [x] Fix auth bug #rv-api [01:20] <!--n id:A2 t:09:34 done:11:02-->
- [ ] Review PR 412 <!--n id:A3 t:09:40 from:2026-09-01 carried:2-->
`
	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(note.Items))
	}

	first := note.Items[0]
	if first.Done {
		t.Error("first item should not be done")
	}
	if first.ID != "A1" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.CreatedAt != "08:55" {
		t.Errorf("CreatedAt = %q", first.CreatedAt)
	}
	if first.Recurring != "daily" {
		t.Errorf("Recurring = %q", first.Recurring)
	}
	if got := first.Labels(); len(got) != 1 || got[0] != "daily" {
		t.Errorf("Labels = %v, want [daily]", got)
	}

	second := note.Items[1]
	if !second.Done {
		t.Error("second item should be done")
	}
	if second.DoneAt != "11:02" {
		t.Errorf("DoneAt = %q", second.DoneAt)
	}
	if second.Minutes() != 80 {
		t.Errorf("Minutes() = %d, want 80", second.Minutes())
	}

	third := note.Items[2]
	if third.From != "2026-09-01" {
		t.Errorf("From = %q", third.From)
	}
	if third.Carried != 2 {
		t.Errorf("Carried = %d, want 2", third.Carried)
	}
}

func TestParseNestedItemsAndBodies(t *testing.T) {
	in := `---
type: workplan
---

- [ ] Parent <!--n id:P t:09:00-->
      A note under the parent.

      ` + "```go" + `
      if exp <= now {
          return ErrExpired
      }
      ` + "```" + `

  - [ ] Child <!--n id:C t:09:05-->
- [ ] Sibling <!--n id:S t:09:10-->
`
	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(note.Items))
	}

	parent := note.Items[0]
	if parent.Depth != 0 {
		t.Errorf("parent Depth = %d, want 0", parent.Depth)
	}
	body := strings.Join(parent.Body, "\n")
	if !strings.Contains(body, "A note under the parent.") {
		t.Errorf("parent body missing prose:\n%s", body)
	}
	if !strings.Contains(body, "return ErrExpired") {
		t.Errorf("parent body missing the code fence:\n%s", body)
	}

	if note.Items[1].Depth != 1 {
		t.Errorf("child Depth = %d, want 1", note.Items[1].Depth)
	}
	if note.Items[2].Depth != 0 {
		t.Errorf("sibling Depth = %d, want 0", note.Items[2].Depth)
	}
}

// A checklist line inside a fenced code block is content, not an action item.
func TestFencedCodeIsNotParsedAsItems(t *testing.T) {
	in := "---\ntype: workplan\n---\n\n" +
		"- [ ] Real item <!--n id:R t:09:00-->\n" +
		"      ```markdown\n" +
		"      - [ ] not a real item\n" +
		"      - [x] also not real\n" +
		"      ```\n"

	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 1 {
		t.Fatalf("got %d items, want 1 — checklist lines inside a code fence must not be parsed as items", len(note.Items))
	}
	if got := Serialize(note); got != in {
		t.Errorf("round trip changed the note.\ngot:\n%s\nwant:\n%s", got, in)
	}
}

func TestLabelsAndTimeStayVisibleInText(t *testing.T) {
	in := "- [ ] Ship it #release [02:15]\n"
	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	item := note.Items[0]

	if !strings.Contains(item.Text, "#release") {
		t.Errorf("label was stripped from the text: %q", item.Text)
	}
	if !strings.Contains(item.Text, "[02:15]") {
		t.Errorf("time was stripped from the text: %q", item.Text)
	}
	if item.Minutes() != 135 {
		t.Errorf("Minutes() = %d, want 135", item.Minutes())
	}
}

func TestSetMinutes(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		minutes int
		want    string
	}{
		{"adds when absent", "Ship it #release", 80, "Ship it #release [01:20]"},
		{"replaces when present", "Ship it [00:30] #release", 80, "Ship it [01:20] #release"},
		{"removes when zero", "Ship it [00:30] #release", 0, "Ship it #release"},
		{"pads hours past nine", "Long day", 600, "Long day [10:00]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := Item{Text: tt.text}
			item.SetMinutes(tt.minutes)
			if item.Text != tt.want {
				t.Errorf("Text = %q, want %q", item.Text, tt.want)
			}
			if got := item.Minutes(); got != tt.minutes {
				t.Errorf("Minutes() = %d, want %d", got, tt.minutes)
			}
		})
	}
}

func TestParseDurationAndFormatDuration(t *testing.T) {
	tests := []struct {
		text    string
		minutes int
		ok      bool
	}{
		{"00:00", 0, true},
		{"09:34", 574, true},
		{"01:20", 80, true},
		{"10:00", 600, true},
		{"99:59", 5999, true},
		{"9:34", 0, false},  // hours must be zero padded
		{"09:60", 0, false}, // minutes out of range
		{"09-34", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, ok := ParseDuration(tt.text)
			if ok != tt.ok {
				t.Fatalf("ParseDuration(%q) ok = %v, want %v", tt.text, ok, tt.ok)
			}
			if !ok {
				return
			}
			if got != tt.minutes {
				t.Errorf("ParseDuration(%q) = %d, want %d", tt.text, got, tt.minutes)
			}
			if back := FormatDuration(got); back != tt.text {
				t.Errorf("FormatDuration(%d) = %q, want %q", got, back, tt.text)
			}
		})
	}
}

func TestNoteLabelsFromFrontmatterAndItems(t *testing.T) {
	in := `---
type: workplan
labels: [work]
---

- [ ] One #alpha <!--n id:1-->
- [ ] Two #beta #alpha <!--n id:2-->
`
	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	got := note.AllLabels()
	want := []string{"alpha", "beta", "work"}
	if len(got) != len(want) {
		t.Fatalf("AllLabels() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllLabels() = %v, want %v (sorted, deduplicated)", got, want)
		}
	}
}

func TestCRLFInputIsNormalised(t *testing.T) {
	in := "---\r\ntype: workplan\r\n---\r\n\r\n- [ ] Windows line endings <!--n id:W-->\r\n"
	note, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(note.Items))
	}
	got := Serialize(note)
	if strings.Contains(got, "\r") {
		t.Error("serialised output still contains carriage returns")
	}
}

func TestMalformedFrontmatterIsAnError(t *testing.T) {
	if _, err := Parse("---\n\tnot: [valid\n---\n"); err == nil {
		t.Error("expected an error for malformed frontmatter")
	}
}

func TestAddAndFindItem(t *testing.T) {
	n := &Note{}
	n.AddItem("X1", "  Do the thing  ", "09:00", 0)

	it := n.FindItem("X1")
	if it == nil {
		t.Fatal("FindItem returned nil")
	}
	if it.Text != "Do the thing" {
		t.Errorf("Text = %q, want it trimmed", it.Text)
	}
	if it.CreatedAt != "09:00" {
		t.Errorf("CreatedAt = %q", it.CreatedAt)
	}
	if n.FindItem("missing") != nil {
		t.Error("FindItem found an item that does not exist")
	}
}

func TestSetDoneStampsAndClears(t *testing.T) {
	n := &Note{}
	n.AddItem("X1", "Thing", "09:00", 0)

	if !n.SetDone("X1", true, "11:02") {
		t.Fatal("SetDone reported no such item")
	}
	if it := n.FindItem("X1"); !it.Done || it.DoneAt != "11:02" {
		t.Errorf("after ticking: done=%v at=%q", it.Done, it.DoneAt)
	}

	n.SetDone("X1", false, "12:00")
	if it := n.FindItem("X1"); it.Done || it.DoneAt != "" {
		t.Errorf("reopening must clear the completion time: done=%v at=%q", it.Done, it.DoneAt)
	}
}

func TestRemoveItemAlsoRemovesItsChildren(t *testing.T) {
	n := &Note{}
	n.AddItem("P", "Parent", "09:00", 0)
	n.AddItem("C1", "Child one", "09:01", 1)
	n.AddItem("C2", "Child two", "09:02", 1)
	n.AddItem("S", "Sibling", "09:03", 0)

	if !n.RemoveItem("P") {
		t.Fatal("RemoveItem reported no such item")
	}
	if len(n.Items) != 1 || n.Items[0].ID != "S" {
		t.Errorf("items after removing the parent = %+v, want only the sibling", n.Items)
	}
}

func TestAddItemMinutesAccumulatesAndFloorsAtZero(t *testing.T) {
	n := &Note{}
	n.AddItem("X1", "Thing", "09:00", 0)

	n.AddItemMinutes("X1", 80)
	if got := n.FindItem("X1").Minutes(); got != 80 {
		t.Errorf("Minutes() = %d, want 80", got)
	}
	n.AddItemMinutes("X1", 40)
	if got := n.FindItem("X1").Minutes(); got != 120 {
		t.Errorf("Minutes() = %d, want 120", got)
	}
	n.AddItemMinutes("X1", -500)
	if got := n.FindItem("X1").Minutes(); got != 0 {
		t.Errorf("Minutes() = %d, want 0 — time logged cannot go negative", got)
	}
}

func TestReplaceItemsKeepsFrontmatterAndBody(t *testing.T) {
	src := "---\ntype: workplan\ndate: 2026-09-02\nhours: \"03:00\"\ndaytype: work\n---\n\n" +
		"- [ ] Old one <!--n id:A1 t:09:00-->\n\n## Notes\n\nKeep me.\n"
	n, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	n.ReplaceItems([]Item{
		{ID: "A1", Text: "Old one, retitled", CreatedAt: "09:00"},
		{ID: "B2", Text: "Brand new", CreatedAt: "10:15", Depth: 1},
	})

	got := Serialize(n)
	want := "---\ntype: workplan\ndate: 2026-09-02\nhours: \"03:00\"\ndaytype: work\n---\n\n" +
		"- [ ] Old one, retitled <!--n id:A1 t:09:00-->\n" +
		"  - [ ] Brand new <!--n id:B2 t:10:15-->\n\n## Notes\n\nKeep me.\n"
	if got != want {
		t.Errorf("ReplaceItems changed more than the items.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Ticking is a transition, not a flag: the completion time is stamped when an
// item becomes done and cleared when it is reopened, whatever the caller sends.
func TestReplaceItemsStampsDoneTransitions(t *testing.T) {
	n := &Note{}
	n.AddItem("A", "Was open", "09:00", 0)
	n.AddItem("B", "Was done", "09:01", 0)
	n.SetDone("B", true, "09:30")

	n.ReplaceItemsAt([]Item{
		{ID: "A", Text: "Was open", Done: true, CreatedAt: "09:00"},  // open -> done
		{ID: "B", Text: "Was done", Done: false, CreatedAt: "09:01"}, // done -> open
	}, "11:45")

	if a := n.FindItem("A"); a.DoneAt != "11:45" {
		t.Errorf("A.DoneAt = %q, want the transition time 11:45", a.DoneAt)
	}
	if b := n.FindItem("B"); b.DoneAt != "" {
		t.Errorf("B.DoneAt = %q, want cleared on reopen", b.DoneAt)
	}
}

func TestReplaceItemsPreservesExistingDoneStamp(t *testing.T) {
	n := &Note{}
	n.AddItem("A", "Done earlier", "09:00", 0)
	n.SetDone("A", true, "09:30")

	// Still done; the stamp must not move to "now".
	n.ReplaceItemsAt([]Item{{ID: "A", Text: "Done earlier", Done: true, CreatedAt: "09:00"}}, "13:00")

	if a := n.FindItem("A"); a.DoneAt != "09:30" {
		t.Errorf("DoneAt = %q, want the original 09:30", a.DoneAt)
	}
}

// Metadata the editor does not send — carry counters, recurring id, original
// creation time — survives a replace keyed on the item id.
func TestReplaceItemsKeepsRolloverMetadataById(t *testing.T) {
	src := "- [ ] Carried #x <!--n id:A1 t:09:40 from:2026-09-01 carried:3 rec:check-calendar-->\n"
	n, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	n.ReplaceItemsAt([]Item{{ID: "A1", Text: "Carried #x, edited"}}, "12:00")

	got := n.FindItem("A1")
	if got.From != "2026-09-01" || got.Carried != 3 || got.Recurring != "check-calendar" || got.CreatedAt != "09:40" {
		t.Errorf("rollover metadata lost: %+v", got)
	}
}
