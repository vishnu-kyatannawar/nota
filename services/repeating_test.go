package services

import (
	"testing"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
)

func repeatingService(t *testing.T) (*WorkplanService, *Core) {
	t.Helper()
	core := newTestCore(t)
	return NewWorkplanService(core), core
}

func TestAddRepeatingReachesTodayNotJustTomorrow(t *testing.T) {
	w, core := repeatingService(t)
	if _, err := w.EnsureToday(); err != nil {
		t.Fatal(err)
	}
	if err := w.AddRepeating("Check email"); err != nil {
		t.Fatal(err)
	}

	path := core.plans.PathFor(time.Now())
	note, err := core.vault.ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range note.Items {
		if it.Text == "Check email" && it.Recurring != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("today does not hold the item just added: %+v", note.Items)
	}

	tpls, err := w.Repeating()
	if err != nil {
		t.Fatal(err)
	}
	if len(tpls) != 1 || tpls[0].Text != "Check email" {
		t.Errorf("Repeating() = %+v", tpls)
	}
}

func TestStopRepeatingTakesItOutOfTodayOnly(t *testing.T) {
	w, core := repeatingService(t)
	if _, err := w.EnsureToday(); err != nil {
		t.Fatal(err)
	}
	if err := w.AddRepeating("Check email"); err != nil {
		t.Fatal(err)
	}
	tpls, _ := w.Repeating()
	id := tpls[0].ID

	// A day already written, holding the same repeating item.
	yesterday := time.Now().AddDate(0, 0, -1)
	past := core.plans.PathFor(yesterday)
	if err := core.vault.WriteNote(past, &mdnote.Note{
		Type: "workplan", Date: yesterday.Format("2006-01-02"), HadFrontmatter: true,
		Items: []mdnote.Item{{ID: "P1", Text: "Check email", Recurring: id, CreatedAt: "09:00"}},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := core.vault.ReadRaw(past)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.StopRepeating(id); err != nil {
		t.Fatal(err)
	}

	after, err := core.vault.ReadRaw(past)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("an earlier workplan was rewritten:\n%s\nwant:\n%s", after, before)
	}

	today, err := core.vault.ReadNote(core.plans.PathFor(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range today.Items {
		if it.Recurring == id {
			t.Error("today still holds it")
		}
	}
	if tpls, _ := w.Repeating(); len(tpls) != 0 {
		t.Errorf("it still repeats: %+v", tpls)
	}
}

func TestEditingTheRowRenamesWhatRepeats(t *testing.T) {
	w, core := repeatingService(t)
	if _, err := w.EnsureToday(); err != nil {
		t.Fatal(err)
	}
	if err := w.AddRepeating("Check email"); err != nil {
		t.Fatal(err)
	}
	path := core.plans.PathFor(time.Now())
	note, err := core.vault.ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	var row mdnote.Item
	for _, it := range note.Items {
		if it.Recurring != "" {
			row = it
		}
	}

	if _, err := w.SaveItems(path, []ItemInput{
		{ID: row.ID, Text: "Check email and Slack"},
	}); err != nil {
		t.Fatal(err)
	}

	tpls, err := w.Repeating()
	if err != nil {
		t.Fatal(err)
	}
	if len(tpls) != 1 {
		t.Fatalf("got %d repeating items, want 1 — a rename must not make a second", len(tpls))
	}
	if tpls[0].Text != "Check email and Slack" {
		t.Errorf("text = %q, want the new one", tpls[0].Text)
	}
	if tpls[0].ID != row.Recurring {
		t.Errorf("id changed from %q to %q", row.Recurring, tpls[0].ID)
	}
}

func TestEditingAPastWorkplanLeavesWhatRepeatsAlone(t *testing.T) {
	w, core := repeatingService(t)
	if err := w.AddRepeating("Check email"); err != nil {
		t.Fatal(err)
	}
	tpls, _ := w.Repeating()
	id := tpls[0].ID

	yesterday := time.Now().AddDate(0, 0, -1)
	past := core.plans.PathFor(yesterday)
	if err := core.vault.WriteNote(past, &mdnote.Note{
		Type: "workplan", Date: yesterday.Format("2006-01-02"), HadFrontmatter: true,
		Items: []mdnote.Item{{ID: "P1", Text: "Check email", Recurring: id, CreatedAt: "09:00"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Correcting what a past day said must not change what happens tomorrow.
	if _, err := w.SaveItems(past, []ItemInput{{ID: "P1", Text: "Checked email twice"}}); err != nil {
		t.Fatal(err)
	}
	after, err := w.Repeating()
	if err != nil {
		t.Fatal(err)
	}
	if after[0].Text != "Check email" {
		t.Errorf("what repeats changed to %q from an edit to a past workplan", after[0].Text)
	}
}
