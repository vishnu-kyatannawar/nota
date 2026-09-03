package services

import (
	"fmt"
	"strings"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

// NotesService is the frontend's access to the vault tree and note contents.
type NotesService struct {
	core *Core
}

// NewNotesService returns the service bound as NotesService.
func NewNotesService(core *Core) *NotesService { return &NotesService{core: core} }

// Note is a note as the editor needs it: its frontmatter, its items, and the
// free-form markdown below them.
type Note struct {
	Path    string     `json:"path"`
	ID      string     `json:"id"`
	Type    string     `json:"type"`
	Date    string     `json:"date"`
	Hours   string     `json:"hours"`
	DayType string     `json:"dayType"`
	Layout  string     `json:"layout"`
	Labels  []string   `json:"labels"`
	Items   []NoteItem `json:"items"`
	Body    string     `json:"body"`
	Title   string     `json:"title"`
}

// NoteItem is one action item or group heading, flattened for the editor.
type NoteItem struct {
	Kind      string   `json:"kind"`
	Level     int      `json:"level"`
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Done      bool     `json:"done"`
	Depth     int      `json:"depth"`
	Minutes   int      `json:"minutes"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"createdAt"`
	DoneAt    string   `json:"doneAt"`
	From      string   `json:"from"`
	Carried   int      `json:"carried"`
	Recurring string   `json:"recurring"`
	Body      []string `json:"body"`
}

// Tree returns the folder and note tree for the sidebar.
func (n *NotesService) Tree() (*vault.Node, error) { return n.core.vault.Tree() }

// Get reads one note.
func (n *NotesService) Get(path string) (*Note, error) {
	note, err := n.core.vault.ReadNote(path)
	if err != nil {
		return nil, err
	}
	return toNote(path, note), nil
}

// GetRaw returns a note's markdown for the raw editor.
func (n *NotesService) GetRaw(path string) (string, error) { return n.core.vault.ReadRaw(path) }

// SaveRaw replaces a note's markdown wholesale, which is what the raw editor does.
func (n *NotesService) SaveRaw(path, content string) error {
	n.core.mu.Lock()
	defer n.core.mu.Unlock()

	note, err := mdnote.Parse(content)
	if err != nil {
		return err
	}
	if err := n.core.vault.WriteRaw(path, content); err != nil {
		return err
	}
	return n.core.index.Update(path, note)
}

// Create makes a new empty note and returns its path. If the path is taken the
// name gets a numeric suffix — "Untitled 2" — rather than overwriting a note the
// user cannot see.
func (n *NotesService) Create(path string) (string, error) {
	n.core.mu.Lock()
	defer n.core.mu.Unlock()

	path = n.unusedPath(path)
	note := &mdnote.Note{ID: newID(), HadFrontmatter: true}
	if err := n.core.saveNote(path, note); err != nil {
		return "", err
	}
	return path, nil
}

func (n *NotesService) unusedPath(path string) string {
	if _, err := n.core.vault.ReadRaw(path); err != nil {
		return path
	}
	stem := strings.TrimSuffix(path, mdnote.Ext)
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s %d%s", stem, i, mdnote.Ext)
		if _, err := n.core.vault.ReadRaw(candidate); err != nil {
			return candidate
		}
	}
	return path
}

// reserved reports whether a path is the workplan folder itself, which must
// keep its name for rollover to find each day's note.
func (n *NotesService) reserved(path string) bool {
	folder := n.core.Settings().WorkplanFolder
	if folder == "" {
		folder = config.DefaultWorkplanFolder
	}
	return strings.Trim(path, "/") == strings.Trim(folder, "/")
}

// CreateFolder makes a folder, including any missing parents.
func (n *NotesService) CreateFolder(path string) error { return n.core.vault.CreateFolder(path) }

// Rename moves a note or folder.
func (n *NotesService) Rename(from, to string) error {
	if n.reserved(from) {
		return fmt.Errorf("%q is the workplan folder and cannot be renamed", from)
	}
	n.core.mu.Lock()
	defer n.core.mu.Unlock()

	if err := n.core.vault.Rename(from, to); err != nil {
		return err
	}
	// The index keys on path, so a move is a remove plus a fresh insert.
	if err := n.core.index.Remove(from); err != nil {
		return err
	}
	if note, err := n.core.vault.ReadNote(to); err == nil {
		return n.core.index.Update(to, note)
	}
	// A folder moved: paths beneath it all changed, so rebuild rather than
	// trying to rewrite each row.
	return n.core.index.Rebuild(n.core.vault)
}

// Delete removes a note, or a folder and everything under it.
func (n *NotesService) Delete(path string) error {
	if n.reserved(path) {
		return fmt.Errorf("%q is the workplan folder and cannot be deleted", path)
	}
	n.core.mu.Lock()
	defer n.core.mu.Unlock()

	if err := n.core.vault.Delete(path); err != nil {
		return err
	}
	if err := n.core.index.Remove(path); err != nil {
		return err
	}
	// A deleted folder takes its notes with it, and only a rebuild knows which.
	return n.core.index.Rebuild(n.core.vault)
}

// SaveBody replaces the free-form markdown below the items and nothing else:
// the frontmatter and every item line are written back exactly as they were.
// An empty body removes the section rather than leaving a blank stub.
func (n *NotesService) SaveBody(path, body string) error {
	body = strings.TrimRight(body, "\n")
	if body != "" {
		body += "\n"
	}
	return n.core.mutate(path, func(note *mdnote.Note) error {
		note.Body = body
		return nil
	})
}

// SetLayout chooses which sections a note shows. Workplans are always items
// then notes, so they refuse a layout rather than silently ignoring it.
func (n *NotesService) SetLayout(path, layout string) error {
	switch layout {
	case mdnote.LayoutItems, mdnote.LayoutNotes, mdnote.LayoutBoth:
	default:
		return fmt.Errorf("unknown layout %q", layout)
	}
	return n.core.mutate(path, func(note *mdnote.Note) error {
		if note.Type == "workplan" {
			return fmt.Errorf("a workplan always shows items and notes")
		}
		if layout == mdnote.LayoutBoth {
			note.Layout = "" // the default; keep the file minimal
		} else {
			note.Layout = layout
		}
		return nil
	})
}

// SetLabels replaces a note's own labels (the frontmatter ones, as opposed to
// the #labels on individual items).
func (n *NotesService) SetLabels(path string, labels []string) error {
	clean := make([]string, 0, len(labels))
	seen := map[string]bool{}
	for _, l := range labels {
		l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "#"))
		l = strings.Join(strings.Fields(l), "-")
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		clean = append(clean, l)
	}
	return n.core.mutate(path, func(note *mdnote.Note) error {
		note.Labels = clean
		return nil
	})
}

// ListTrash returns the recoverable deleted notes and folders, newest first.
func (n *NotesService) ListTrash() ([]vault.TrashEntry, error) {
	entries, err := n.core.vault.ListTrash()
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []vault.TrashEntry{}
	}
	return entries, nil
}

// Restore brings a trashed note or folder back and returns where it landed.
func (n *NotesService) Restore(id string) (string, error) {
	n.core.mu.Lock()
	defer n.core.mu.Unlock()

	path, err := n.core.vault.Restore(id)
	if err != nil {
		return "", err
	}
	// A restored folder brings back paths only the walker knows about.
	if err := n.core.index.Rebuild(n.core.vault); err != nil {
		return "", err
	}
	return path, nil
}

// DeleteForever removes one trash entry permanently.
func (n *NotesService) DeleteForever(id string) error { return n.core.vault.DeleteForever(id) }

// EmptyTrash removes every trash entry permanently.
func (n *NotesService) EmptyTrash() error { return n.core.vault.EmptyTrash() }

// Reindex rebuilds the index from the notes on disk. It backs the file watcher
// and the "something changed underneath us" recovery path.
func (n *NotesService) Reindex() error {
	n.core.mu.Lock()
	defer n.core.mu.Unlock()
	return n.core.index.Rebuild(n.core.vault)
}

func toNote(path string, n *mdnote.Note) *Note {
	out := &Note{
		Path:    path,
		ID:      n.ID,
		Type:    n.Type,
		Date:    n.Date,
		Hours:   n.Hours,
		DayType: n.DayType,
		Layout:  n.EffectiveLayout(),
		Labels:  n.Labels,
		Body:    n.Body,
		Title:   titleFor(path, n),
		Items:   make([]NoteItem, 0, len(n.Items)),
	}
	if out.Labels == nil {
		out.Labels = []string{}
	}
	for _, it := range n.Items {
		labels := it.Labels()
		if labels == nil {
			labels = []string{}
		}
		body := it.Body
		if body == nil {
			body = []string{}
		}
		out.Items = append(out.Items, NoteItem{
			Kind:      it.Kind,
			Level:     it.Level,
			ID:        it.ID,
			Text:      it.Text,
			Done:      it.Done,
			Depth:     it.Depth,
			Minutes:   it.Minutes(),
			Labels:    labels,
			CreatedAt: it.CreatedAt,
			DoneAt:    it.DoneAt,
			From:      it.From,
			Carried:   it.Carried,
			Recurring: it.Recurring,
			Body:      body,
		})
	}
	return out
}

// titleFor is what the note is called in the UI. A workplan is named for its
// date and the hours worked that day — "2026-09-02 - 09:00" — while the filename
// itself stays just the date, so logging time never renames the file.
func titleFor(path string, n *mdnote.Note) string {
	base := baseName(path)
	if n.Type == "workplan" && n.Date != "" {
		hours := n.Hours
		if hours == "" {
			hours = mdnote.FormatDuration(0)
		}
		return n.Date + " - " + hours
	}
	return base
}

func baseName(path string) string {
	name := path
	if i := lastIndex(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	if len(name) > len(mdnote.Ext) && name[len(name)-len(mdnote.Ext):] == mdnote.Ext {
		name = name[:len(name)-len(mdnote.Ext)]
	}
	return name
}

func lastIndex(s string, r byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == r {
			return i
		}
	}
	return -1
}
