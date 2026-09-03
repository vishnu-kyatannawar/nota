package services

import (
	"fmt"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/index"
	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/workplan"
)

// WorkplanService is the frontend's access to the dated daily notes and to
// editing action items.
type WorkplanService struct {
	core *Core
}

// NewWorkplanService returns the service bound as WorkplanService.
func NewWorkplanService(core *Core) *WorkplanService { return &WorkplanService{core: core} }

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
func (w *WorkplanService) List() ([]index.Workplan, error) { return w.core.index.Workplans() }

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

// SuggestedMinutes totals the time logged against a day's items. It pre-fills
// the hours field and is never written in automatically, since much of a working
// day never lands on an action item.
func (w *WorkplanService) SuggestedMinutes(path string) (int, error) {
	return w.core.plans.SuggestedMinutes(path)
}

// AddItem appends an action item to a note and returns its new id.
func (w *WorkplanService) AddItem(path, text string, depth int) (string, error) {
	if depth < 0 {
		depth = 0
	}
	id := newID()
	err := w.core.mutate(path, func(n *mdnote.Note) error {
		n.AddItem(id, text, time.Now().Format("15:04"), depth)
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// SetItemDone ticks or unticks an item, stamping the completion time.
func (w *WorkplanService) SetItemDone(path, id string, done bool) error {
	return w.core.mutate(path, func(n *mdnote.Note) error {
		if !n.SetDone(id, done, time.Now().Format("15:04")) {
			return notFound(id)
		}
		return nil
	})
}

// SetItemText replaces an item's visible text, labels and time token included.
func (w *WorkplanService) SetItemText(path, id, text string) error {
	return w.core.mutate(path, func(n *mdnote.Note) error {
		if !n.SetItemText(id, text) {
			return notFound(id)
		}
		return nil
	})
}

// AddItemMinutes adds to the time logged against an item.
func (w *WorkplanService) AddItemMinutes(path, id string, minutes int) error {
	return w.core.mutate(path, func(n *mdnote.Note) error {
		if !n.AddItemMinutes(id, minutes) {
			return notFound(id)
		}
		return nil
	})
}

// RemoveItem deletes an item and anything nested beneath it.
func (w *WorkplanService) RemoveItem(path, id string) error {
	return w.core.mutate(path, func(n *mdnote.Note) error {
		if !n.RemoveItem(id) {
			return notFound(id)
		}
		return nil
	})
}

// SetItemBody replaces the free-form markdown under an item, which is where
// code, JSON and prose belonging to that item live.
func (w *WorkplanService) SetItemBody(path, id string, body []string) error {
	return w.core.mutate(path, func(n *mdnote.Note) error {
		it := n.FindItem(id)
		if it == nil {
			return notFound(id)
		}
		it.Body = body
		return nil
	})
}

// Templates returns the configured recurring items.
func (w *WorkplanService) Templates() ([]workplan.Template, error) {
	return w.core.plans.Templates()
}

// SaveTemplates replaces the recurring items file with the given lines.
func (w *WorkplanService) SaveTemplates(content string) error {
	w.core.mu.Lock()
	defer w.core.mu.Unlock()
	return w.core.vault.WriteRaw(workplan.TemplatePath, content)
}
