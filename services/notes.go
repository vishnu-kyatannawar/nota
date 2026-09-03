package services

import (
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
	Labels  []string   `json:"labels"`
	Items   []NoteItem `json:"items"`
	Body    string     `json:"body"`
	Title   string     `json:"title"`
}

// NoteItem is one action item, flattened for the editor.
type NoteItem struct {
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

// Create makes a new empty note and returns its path.
func (n *NotesService) Create(path string) (string, error) {
	n.core.mu.Lock()
	defer n.core.mu.Unlock()

	note := &mdnote.Note{ID: newID(), HadFrontmatter: true}
	if err := n.core.saveNote(path, note); err != nil {
		return "", err
	}
	return path, nil
}

// CreateFolder makes a folder, including any missing parents.
func (n *NotesService) CreateFolder(path string) error { return n.core.vault.CreateFolder(path) }

// Rename moves a note or folder.
func (n *NotesService) Rename(from, to string) error {
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
