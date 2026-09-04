package services

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/index"
	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/workplan"
)

// WorkplanService is the frontend's access to the dated daily notes and to
// saving a note's action items.
type WorkplanService struct {
	core *Core
}

// NewWorkplanService returns the service bound as WorkplanService.
func NewWorkplanService(core *Core) *WorkplanService { return &WorkplanService{core: core} }

// ItemInput is an action item as the editor holds it: only the fields a person
// can change. Timestamps, carry counters and recurring ids are metadata the
// editor never sees, and are preserved by id on save.
type ItemInput struct {
	Kind  string   `json:"kind"`
	Level int      `json:"level"`
	ID    string   `json:"id"`
	Text  string   `json:"text"`
	Done  bool     `json:"done"`
	Depth int      `json:"depth"`
	Body  []string `json:"body"`
}

// EnsureToday creates today's workplan if it is missing, rolling unfinished
// items forward, and returns its path. An empty path means today is a weekend
// and weekend notes are switched off.
func (w *WorkplanService) EnsureToday() (string, error) {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()

	path, err := w.core.plans.Ensure(time.Now())
	if err != nil || path == "" {
		return path, err
	}
	note, err := w.core.vault.ReadNote(path)
	if err != nil {
		return "", err
	}
	if err := w.core.index.Update(path, note); err != nil {
		return "", err
	}
	return path, nil
}

// List returns the workplans newest first, for the sidebar.
func (w *WorkplanService) List() ([]index.Workplan, error) {
	plans, err := w.core.index.Workplans()
	if err != nil {
		return nil, err
	}
	if plans == nil {
		plans = []index.Workplan{}
	}
	return plans, nil
}

// SaveItems replaces a note's action items in one write. This is the only way
// the editor changes items: it edits a local copy and sends the whole list, so
// keyboard navigation and multi-line paste never wait on a round trip. Items
// with no id are new and get one minted here, stamped with the current time.
func (w *WorkplanService) SaveItems(path string, items []ItemInput) ([]mdnote.Item, error) {
	now := time.Now().Format("15:04")
	incoming := make([]mdnote.Item, 0, len(items))
	for _, in := range items {
		it := mdnote.Item{Kind: in.Kind, Level: in.Level, ID: in.ID, Text: in.Text, Done: in.Done, Depth: in.Depth, Body: in.Body}
		if it.Kind == mdnote.KindHeading {
			incoming = append(incoming, it) // headings carry no id or timestamp
			continue
		}
		if it.ID == "" {
			it.ID = newID()
			it.CreatedAt = now
		}
		incoming = append(incoming, it)
	}

	var saved []mdnote.Item
	err := w.core.mutate(path, func(n *mdnote.Note) error {
		n.ReplaceItemsAt(incoming, now)
		saved = n.Items
		return nil
	})
	if err != nil {
		return nil, err
	}
	w.syncRepeatingText(path, saved)
	if saved == nil {
		saved = []mdnote.Item{}
	}
	return saved, nil
}

// MoveItems takes the named items — each with anything nested under it — out
// of one note and appends them to another. An empty destination means today's
// workplan, created if needed. Ids, creation times, carry counters and bodies
// travel untouched, so an item moved in from a list still says when it was
// first written down. Both notes are saved and reindexed under one lock.
func (w *WorkplanService) MoveItems(from string, ids []string, to string) (string, error) {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()

	if to == "" {
		path, err := w.core.plans.Ensure(time.Now())
		if err != nil {
			return "", err
		}
		if path == "" {
			return "", fmt.Errorf("no workplan for today: weekend notes are switched off")
		}
		to = path
	}
	if to == from {
		return "", fmt.Errorf("items are already in %s", to)
	}

	source, err := w.core.vault.ReadNote(from)
	if err != nil {
		return "", err
	}
	dest, err := w.core.vault.ReadNote(to)
	if err != nil {
		return "", err
	}

	moved := source.TakeItems(ids)
	if len(moved) == 0 {
		return to, nil
	}
	dest.AppendItems(moved)

	if err := w.core.saveNote(to, dest); err != nil {
		return "", err
	}
	if err := w.core.saveNote(from, source); err != nil {
		return "", err
	}
	return to, nil
}

// SetHours records the hours worked on a day, in hh:mm.
func (w *WorkplanService) SetHours(path, hours string) error {
	if _, ok := mdnote.ParseDuration(hours); !ok {
		return fmt.Errorf("%w: got %q", workplan.ErrInvalidHours, hours)
	}
	return w.core.mutate(path, func(n *mdnote.Note) error {
		n.Hours = hours
		return nil
	})
}

// SetDayType marks a day work, weekend, leave or holiday.
func (w *WorkplanService) SetDayType(path, dayType string) error {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()

	if err := w.core.plans.SetDayType(path, dayType); err != nil {
		return err
	}
	note, err := w.core.vault.ReadNote(path)
	if err != nil {
		return err
	}
	return w.core.index.Update(path, note)
}

// Repeating returns the items that repeat, for the section at the top of a
// workplan.
func (w *WorkplanService) Repeating() ([]workplan.Template, error) {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()
	tpls, err := w.core.plans.Templates()
	if err != nil {
		return nil, err
	}
	if tpls == nil {
		tpls = []workplan.Template{}
	}
	return tpls, nil
}

// AddRepeating makes an item repeat every day, and puts it into today as well —
// waiting until tomorrow to see something you just added would be strange.
func (w *WorkplanService) AddRepeating(text string) error {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()

	if _, err := w.core.plans.AddTemplate(text); err != nil {
		return err
	}
	return w.seedToday()
}

// StopRepeating stops an item repeating and takes it out of today. Workplans
// already written keep their copy: what happened on a day is a record of that
// day, not something to revise.
func (w *WorkplanService) StopRepeating(id string) error {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()

	if err := w.core.plans.RemoveTemplate(id); err != nil {
		return err
	}
	today := time.Now()
	if err := w.core.plans.DropFrom(today, id); err != nil {
		return err
	}
	return w.reindex(w.core.plans.PathFor(today))
}

// seedToday adds anything newly due to today's note, if today has one.
func (w *WorkplanService) seedToday() error {
	today := time.Now()
	path := w.core.plans.PathFor(today)
	if _, err := w.core.vault.ReadNote(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// A weekend with notes switched off, say. Tomorrow's note seeds it.
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := w.core.plans.SeedInto(today); err != nil {
		return err
	}
	return w.reindex(path)
}

func (w *WorkplanService) reindex(path string) error {
	note, err := w.core.vault.ReadNote(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return w.core.index.Update(path, note)
}

// syncRepeatingText keeps the repeating item in step when its text is edited in
// today's workplan. Editing the row is how you rename what repeats, so it has
// to work wherever the edit came from — the row, raw markdown, another editor.
//
// Only today: an edit to an earlier workplan is a correction to that day's
// record, not an instruction about what to do tomorrow.
func (w *WorkplanService) syncRepeatingText(path string, items []mdnote.Item) {
	if path != w.core.plans.PathFor(time.Now()) {
		return
	}
	tpls, err := w.core.plans.Templates()
	if err != nil {
		return
	}
	byID := make(map[string]string, len(tpls))
	for _, t := range tpls {
		byID[t.ID] = t.Text
	}
	for _, it := range items {
		if it.Recurring == "" {
			continue
		}
		if was, ok := byID[it.Recurring]; ok && was != it.Text && strings.TrimSpace(it.Text) != "" {
			// Best effort: a failure here must not fail the save of the note.
			_ = w.core.plans.RenameTemplate(it.Recurring, it.Text)
		}
	}
}

// Templates returns the configured recurring items.
func (w *WorkplanService) Templates() ([]workplan.Template, error) {
	tpls, err := w.core.plans.Templates()
	if err != nil {
		return nil, err
	}
	if tpls == nil {
		tpls = []workplan.Template{}
	}
	return tpls, nil
}

// SaveTemplates replaces the recurring items file with the given lines.
func (w *WorkplanService) SaveTemplates(content string) error {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()
	return w.core.vault.WriteRaw(workplan.TemplatePath, content)
}
