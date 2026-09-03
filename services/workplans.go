package services

import (
	"fmt"
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
		it := mdnote.Item{ID: in.ID, Text: in.Text, Done: in.Done, Depth: in.Depth, Body: in.Body}
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
	if saved == nil {
		saved = []mdnote.Item{}
	}
	return saved, nil
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
