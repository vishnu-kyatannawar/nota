package workplan

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func newManager(t *testing.T) (*Manager, *vault.Vault) {
	t.Helper()
	v, err := vault.Open(filepath.Join(t.TempDir(), "Notes"))
	if err != nil {
		t.Fatal(err)
	}
	return New(v, Options{Folder: "Workplans", CreateOnWeekends: true}), v
}

func write(t *testing.T, v *vault.Vault, path, content string) {
	t.Helper()
	if err := v.WriteRaw(path, content); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, v *vault.Vault, path string) string {
	t.Helper()
	s, err := v.ReadRaw(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return s
}

func TestEnsureCreatesTodaysNoteWithZeroHours(t *testing.T) {
	m, v := newManager(t)

	path, err := m.Ensure(date("2026-09-02")) // a Wednesday
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if path != "Workplans/2026-09-02.md" {
		t.Errorf("path = %q", path)
	}

	note, err := v.ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	if note.Hours != "00:00" {
		t.Errorf("Hours = %q, want 00:00 — a new day starts at zero", note.Hours)
	}
	if note.DayType != "work" {
		t.Errorf("DayType = %q, want work", note.DayType)
	}
	if note.Date != "2026-09-02" {
		t.Errorf("Date = %q", note.Date)
	}
	if note.Type != "workplan" {
		t.Errorf("Type = %q", note.Type)
	}
}

func TestEnsureMarksWeekends(t *testing.T) {
	m, v := newManager(t)

	path, err := m.Ensure(date("2026-09-05")) // a Saturday
	if err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	if note.DayType != "weekend" {
		t.Errorf("DayType = %q, want weekend", note.DayType)
	}
	if note.Hours != "00:00" {
		t.Errorf("Hours = %q, want 00:00", note.Hours)
	}
}

func TestEnsureSkipsWeekendsWhenConfiguredTo(t *testing.T) {
	v, err := vault.Open(filepath.Join(t.TempDir(), "Notes"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(v, Options{Folder: "Workplans", CreateOnWeekends: false})

	path, err := m.Ensure(date("2026-09-05")) // Saturday
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("path = %q, want empty — no note should be created on a weekend", path)
	}
	if _, err := v.ReadNote("Workplans/2026-09-05.md"); err == nil {
		t.Error("a weekend note was created anyway")
	}
}

func TestEnsureIsIdempotent(t *testing.T) {
	m, v := newManager(t)

	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	// The user then edits the day: logs hours and adds an item.
	write(t, v, "Workplans/2026-09-02.md", `---
type: workplan
date: 2026-09-02
hours: "03:15"
daytype: work
---

- [ ] Typed by hand <!--n id:H1 t:10:00-->
`)
	before := read(t, v, "Workplans/2026-09-02.md")

	for i := 0; i < 3; i++ {
		if _, err := m.Ensure(date("2026-09-02")); err != nil {
			t.Fatal(err)
		}
	}

	if after := read(t, v, "Workplans/2026-09-02.md"); after != before {
		t.Errorf("Ensure rewrote an existing day.\n--- after ---\n%s\n--- before ---\n%s", after, before)
	}
}

func TestRolloverCarriesUndoneAndDropsDone(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-01.md", `---
type: workplan
date: 2026-09-01
hours: "09:00"
daytype: work
---

- [x] Fix the auth bug #rv-api [01:20] <!--n id:A1 t:09:34 done:16:11-->
- [x] Check calendar #daily <!--n id:A2 t:08:52 rec:daily done:08:58-->
- [ ] Review PR 412 #rv-portal <!--n id:A3 t:09:40-->
- [ ] Chase the invoice #billing <!--n id:A4 t:14:02-->
`)

	path, err := m.Ensure(date("2026-09-02"))
	if err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(note.Items) != 2 {
		t.Fatalf("got %d items, want the 2 undone ones: %+v", len(note.Items), note.Items)
	}
	if note.Items[0].ID != "A3" || note.Items[1].ID != "A4" {
		t.Errorf("carried the wrong items: %+v", note.Items)
	}
	for _, it := range note.Items {
		if it.Done {
			t.Errorf("carried item %s arrived already done", it.ID)
		}
		if it.DoneAt != "" {
			t.Errorf("carried item %s kept a completion time", it.ID)
		}
	}
}

// An item keeps the time it was originally added, which is the whole point of
// being able to see how long something has been nagging.
func TestCarriedItemsKeepTheirOriginalCreationTime(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-01.md", `---
type: workplan
date: 2026-09-01
hours: "09:00"
daytype: work
---

- [ ] Review PR 412 <!--n id:A3 t:09:40-->
`)

	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}

	it := note.Items[0]
	if it.CreatedAt != "09:40" {
		t.Errorf("CreatedAt = %q, want the original 09:40", it.CreatedAt)
	}
	if it.From != "2026-09-01" {
		t.Errorf("From = %q, want 2026-09-01", it.From)
	}
	if it.Carried != 1 {
		t.Errorf("Carried = %d, want 1", it.Carried)
	}
}

func TestCarryCountIncrementsAcrossDays(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-01.md", `---
type: workplan
date: 2026-09-01
hours: "09:00"
daytype: work
---

- [ ] Long runner <!--n id:L1 t:09:40-->
`)

	for _, d := range []string{"2026-09-02", "2026-09-03", "2026-09-04"} {
		if _, err := m.Ensure(date(d)); err != nil {
			t.Fatalf("Ensure(%s): %v", d, err)
		}
	}

	note, err := v.ReadNote("Workplans/2026-09-04.md")
	if err != nil {
		t.Fatal(err)
	}
	it := note.Items[0]
	if it.Carried != 3 {
		t.Errorf("Carried = %d, want 3", it.Carried)
	}
	if it.From != "2026-09-01" {
		t.Errorf("From = %q, want the first day it appeared", it.From)
	}
	if it.CreatedAt != "09:40" {
		t.Errorf("CreatedAt = %q, want 09:40", it.CreatedAt)
	}
}

// After a weekend, Monday carries from Friday. The rule is "the most recent
// workplan", never "yesterday".
func TestRolloverSkipsGaps(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-04.md", `---
type: workplan
date: 2026-09-04
hours: "08:00"
daytype: work
---

- [ ] Friday leftover <!--n id:F1 t:16:00-->
`)

	// Monday the 7th; the 5th and 6th are the weekend and have no notes.
	if _, err := m.Ensure(date("2026-09-07")); err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-07.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 1 || note.Items[0].ID != "F1" {
		t.Errorf("Monday did not carry from Friday: %+v", note.Items)
	}
}

func TestRolloverCarriesItemBodiesAndNesting(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-01.md", "---\ntype: workplan\ndate: 2026-09-01\nhours: \"09:00\"\ndaytype: work\n---\n\n"+
		"- [ ] Parent <!--n id:P1 t:09:00-->\n"+
		"      Some context worth keeping.\n"+
		"\n"+
		"      ```go\n"+
		"      if exp <= now {\n"+
		"          return ErrExpired\n"+
		"      }\n"+
		"      ```\n"+
		"\n"+
		"  - [ ] Child <!--n id:C1 t:09:05-->\n")

	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	got := read(t, v, "Workplans/2026-09-02.md")

	if !strings.Contains(got, "Some context worth keeping.") {
		t.Errorf("item body was not carried:\n%s", got)
	}
	if !strings.Contains(got, "return ErrExpired") {
		t.Errorf("code block in the body was not carried:\n%s", got)
	}
	if !strings.Contains(got, "  - [ ] Child") {
		t.Errorf("nesting was not preserved:\n%s", got)
	}
}

func TestRolloverKeepsLoggedTimeOnCarriedItems(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-01.md", `---
type: workplan
date: 2026-09-01
hours: "09:00"
daytype: work
---

- [ ] Half done #proj [02:30] <!--n id:H1 t:09:00-->
`)

	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := note.Items[0].Minutes(); got != 150 {
		t.Errorf("Minutes() = %d, want the 150 already logged", got)
	}
	if !strings.Contains(note.Items[0].Text, "#proj") {
		t.Errorf("label was lost: %q", note.Items[0].Text)
	}
}

func TestRecurringItemsAreSeeded(t *testing.T) {
	m, v := newManager(t)
	write(t, v, ".nota/templates/recurring.md", `- [ ] Check calendar #daily @daily
- [ ] Log the day bill #billing @weekdays
- [ ] Weekly report @weekly:fri
`)

	// Wednesday: daily and weekdays apply, the Friday weekly does not.
	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 2 {
		t.Fatalf("got %d seeded items, want 2: %+v", len(note.Items), note.Items)
	}
	for _, it := range note.Items {
		if it.Recurring == "" {
			t.Errorf("seeded item %q has no recurring id", it.Text)
		}
		if strings.Contains(it.Text, "@") {
			t.Errorf("cadence token was left in the text: %q", it.Text)
		}
		if it.CreatedAt == "" {
			t.Errorf("seeded item %q has no creation time", it.Text)
		}
	}
}

func TestRecurringWeeklyOnlyOnItsDay(t *testing.T) {
	m, v := newManager(t)
	write(t, v, ".nota/templates/recurring.md", "- [ ] Weekly report @weekly:fri\n")

	if _, err := m.Ensure(date("2026-09-04")); err != nil { // Friday
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-04.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 1 {
		t.Errorf("Friday should have the weekly item, got %+v", note.Items)
	}
}

func TestRecurringNotSeededOnWeekendsForWeekdayCadence(t *testing.T) {
	m, v := newManager(t)
	write(t, v, ".nota/templates/recurring.md", "- [ ] Log the day bill @weekdays\n")

	if _, err := m.Ensure(date("2026-09-05")); err != nil { // Saturday
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-05.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 0 {
		t.Errorf("a weekday item was seeded on Saturday: %+v", note.Items)
	}
}

// Deleting a recurring item for one day must not make it reappear that same day.
func TestRecurringIsNotReseededWithinTheSameDay(t *testing.T) {
	m, v := newManager(t)
	write(t, v, ".nota/templates/recurring.md", "- [ ] Check calendar @daily\n")

	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	// The user deletes it for today.
	write(t, v, "Workplans/2026-09-02.md", `---
type: workplan
date: 2026-09-02
hours: "00:00"
daytype: work
---
`)
	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 0 {
		t.Errorf("recurring item came back the same day: %+v", note.Items)
	}
}

// A recurring item left undone yesterday should carry, not arrive twice.
func TestRecurringCarriedItemIsNotDuplicated(t *testing.T) {
	m, v := newManager(t)
	write(t, v, ".nota/templates/recurring.md", "- [ ] Check calendar @daily\n")

	if _, err := m.Ensure(date("2026-09-01")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}

	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Items) != 1 {
		t.Errorf("got %d items, want 1 — the carried one, not a carried plus a fresh seed: %+v", len(note.Items), note.Items)
	}
}

func TestSetHoursAndDayType(t *testing.T) {
	m, v := newManager(t)
	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}

	if err := m.SetHours("Workplans/2026-09-02.md", "07:30"); err != nil {
		t.Fatalf("SetHours: %v", err)
	}
	if err := m.SetDayType("Workplans/2026-09-02.md", "leave"); err != nil {
		t.Fatalf("SetDayType: %v", err)
	}

	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.Hours != "07:30" {
		t.Errorf("Hours = %q", note.Hours)
	}
	if note.DayType != "leave" {
		t.Errorf("DayType = %q", note.DayType)
	}
}

func TestSetHoursRejectsABadFormat(t *testing.T) {
	m, _ := newManager(t)
	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"7:30", "0730", "07:60", "", "seven"} {
		if err := m.SetHours("Workplans/2026-09-02.md", bad); err == nil {
			t.Errorf("SetHours(%q) was accepted", bad)
		}
	}
}

func TestSetDayTypeRejectsAnUnknownValue(t *testing.T) {
	m, _ := newManager(t)
	if _, err := m.Ensure(date("2026-09-02")); err != nil {
		t.Fatal(err)
	}
	if err := m.SetDayType("Workplans/2026-09-02.md", "vacation"); err == nil {
		t.Error("an unknown day type was accepted")
	}
}

// The sum of item logs is a suggestion the user can accept, never something that
// silently overwrites a figure they set themselves.
func TestSuggestedMinutesSumsItemLogs(t *testing.T) {
	m, v := newManager(t)
	write(t, v, "Workplans/2026-09-02.md", `---
type: workplan
date: 2026-09-02
hours: "00:00"
daytype: work
---

- [x] One [01:20] <!--n id:S1 t:09:00-->
- [ ] Two [03:40] <!--n id:S2 t:10:00-->
- [ ] Three <!--n id:S3 t:11:00-->
`)

	got, err := m.SuggestedMinutes("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != 300 {
		t.Errorf("SuggestedMinutes() = %d, want 300", got)
	}

	note, err := v.ReadNote("Workplans/2026-09-02.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.Hours != "00:00" {
		t.Errorf("Hours = %q — the suggestion must not write itself in", note.Hours)
	}
}

func TestPathForDate(t *testing.T) {
	m, _ := newManager(t)
	if got := m.PathFor(date("2026-09-02")); got != "Workplans/2026-09-02.md" {
		t.Errorf("PathFor() = %q", got)
	}
}
