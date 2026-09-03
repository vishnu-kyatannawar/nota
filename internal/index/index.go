// Package index keeps a queryable cache of the vault in SQLite.
//
// The markdown files are the source of truth; this database is derived and
// disposable. Deleting it and calling Rebuild must reproduce it exactly, which
// is what lets export be a plain copy of the notes directory and lets a corrupt
// index be fixed by throwing it away. A test asserts a rebuilt index is
// byte-identical to one built by incremental updates, so the cheap path cannot
// quietly drift from the authoritative one.
//
// modernc.org/sqlite is used rather than a cgo binding: Wails already forces one
// cgo dependency and a second would complicate the build for no benefit.
package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver

	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

const schema = `
CREATE TABLE IF NOT EXISTS notes (
    path     TEXT PRIMARY KEY,
    id       TEXT NOT NULL DEFAULT '',
    type     TEXT NOT NULL DEFAULT '',
    date     TEXT NOT NULL DEFAULT '',
    hours    TEXT NOT NULL DEFAULT '',
    minutes  INTEGER NOT NULL DEFAULT 0,
    daytype  TEXT NOT NULL DEFAULT '',
    content  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS items (
    path       TEXT NOT NULL,
    position   INTEGER NOT NULL,
    item_id    TEXT NOT NULL DEFAULT '',
    text       TEXT NOT NULL DEFAULT '',
    done       INTEGER NOT NULL DEFAULT 0,
    minutes    INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT '',
    done_at    TEXT NOT NULL DEFAULT '',
    from_date  TEXT NOT NULL DEFAULT '',
    carried    INTEGER NOT NULL DEFAULT 0,
    recurring  TEXT NOT NULL DEFAULT '',
    depth      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (path, position)
);

CREATE TABLE IF NOT EXISTS labels (
    name    TEXT NOT NULL,
    path    TEXT NOT NULL,
    item_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (name, path, item_id)
);

CREATE INDEX IF NOT EXISTS idx_notes_date  ON notes(date);
CREATE INDEX IF NOT EXISTS idx_items_path  ON items(path);
CREATE INDEX IF NOT EXISTS idx_labels_name ON labels(name);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(path UNINDEXED, content);
`

// Index is an open SQLite cache of the vault.
type Index struct {
	db *sql.DB
}

// Hit is a search result.
type Hit struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

// Label is a label and how many places it appears.
type Label struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Item is an indexed action item.
type Item struct {
	Path      string `json:"path"`
	Position  int    `json:"position"`
	ID        string `json:"id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	Minutes   int    `json:"minutes"`
	CreatedAt string `json:"createdAt"`
	DoneAt    string `json:"doneAt"`
	From      string `json:"from"`
	Carried   int    `json:"carried"`
	Recurring string `json:"recurring"`
	Depth     int    `json:"depth"`
}

// Workplan summarises a dated daily note.
type Workplan struct {
	Path    string `json:"path"`
	Date    string `json:"date"`
	Hours   string `json:"hours"`
	Minutes int    `json:"minutes"`
	DayType string `json:"dayType"`
	Open    int    `json:"open"`
	Done    int    `json:"done"`
}

// Open prepares the index at path, creating the file and schema if needed.
func Open(path string) (*Index, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating index directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening index: %w", err)
	}
	// The index is written from one place; more connections only invite lock
	// contention on a file that is cheap to rebuild anyway.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating index schema: %w", err)
	}
	return &Index{db: db}, nil
}

// Close releases the database.
func (i *Index) Close() error {
	if i.db == nil {
		return nil
	}
	return i.db.Close()
}

// Rebuild discards the index and reindexes every note in the vault.
func (i *Index) Rebuild(v *vault.Vault) error {
	paths, err := v.ListNotes()
	if err != nil {
		return err
	}

	tx, err := i.db.Begin()
	if err != nil {
		return fmt.Errorf("starting rebuild: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range []string{"notes", "items", "labels", "notes_fts"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clearing %s: %w", table, err)
		}
	}

	for _, p := range paths {
		note, err := v.ReadNote(p)
		if err != nil {
			// One unparseable note must not stop the whole index from building.
			continue
		}
		if err := insertNote(tx, p, note); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing rebuild: %w", err)
	}
	return nil
}

// Update reindexes a single note, replacing whatever was there before.
func (i *Index) Update(path string, note *mdnote.Note) error {
	tx, err := i.db.Begin()
	if err != nil {
		return fmt.Errorf("starting update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := deleteNote(tx, path); err != nil {
		return err
	}
	if err := insertNote(tx, path, note); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing update: %w", err)
	}
	return nil
}

// Remove drops a note and every row derived from it.
func (i *Index) Remove(path string) error {
	tx, err := i.db.Begin()
	if err != nil {
		return fmt.Errorf("starting remove: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := deleteNote(tx, path); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing remove: %w", err)
	}
	return nil
}

func deleteNote(tx *sql.Tx, path string) error {
	for _, stmt := range []string{
		"DELETE FROM notes     WHERE path = ?",
		"DELETE FROM items     WHERE path = ?",
		"DELETE FROM labels    WHERE path = ?",
		"DELETE FROM notes_fts WHERE path = ?",
	} {
		if _, err := tx.Exec(stmt, path); err != nil {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}
	return nil
}

func insertNote(tx *sql.Tx, path string, note *mdnote.Note) error {
	minutes, _ := mdnote.ParseDuration(note.Hours)
	content := searchText(note)

	if _, err := tx.Exec(
		`INSERT INTO notes (path, id, type, date, hours, minutes, daytype, content)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		path, note.ID, note.Type, note.Date, note.Hours, minutes, note.DayType, content,
	); err != nil {
		return fmt.Errorf("indexing %s: %w", path, err)
	}
	if _, err := tx.Exec(`INSERT INTO notes_fts (path, content) VALUES (?, ?)`, path, content); err != nil {
		return fmt.Errorf("indexing text of %s: %w", path, err)
	}

	for pos, it := range note.Items {
		if it.Kind == mdnote.KindHeading {
			continue // headings group items; they are not items themselves
		}
		done := 0
		if it.Done {
			done = 1
		}
		if _, err := tx.Exec(
			`INSERT INTO items (path, position, item_id, text, done, minutes, created_at, done_at, from_date, carried, recurring, depth)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			path, pos, it.ID, it.Text, done, it.Minutes(), it.CreatedAt, it.DoneAt, it.From, it.Carried, it.Recurring, it.Depth,
		); err != nil {
			return fmt.Errorf("indexing item %d of %s: %w", pos, path, err)
		}
		for _, label := range it.Labels() {
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO labels (name, path, item_id) VALUES (?, ?, ?)`,
				label, path, it.ID,
			); err != nil {
				return fmt.Errorf("indexing label %q: %w", label, err)
			}
		}
	}

	for _, label := range note.Labels {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO labels (name, path, item_id) VALUES (?, ?, '')`,
			label, path,
		); err != nil {
			return fmt.Errorf("indexing note label %q: %w", label, err)
		}
	}
	return nil
}

// searchText flattens a note into the text that full-text search matches against.
func searchText(note *mdnote.Note) string {
	var b strings.Builder
	for _, it := range note.Items {
		b.WriteString(it.Text)
		b.WriteString("\n")
		for _, line := range it.Body {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString(note.Body)
	return b.String()
}

// ItemsIn returns the action items of one note in file order.
func (i *Index) ItemsIn(path string) ([]Item, error) {
	rows, err := i.db.Query(
		`SELECT path, position, item_id, text, done, minutes, created_at, done_at, from_date, carried, recurring, depth
		 FROM items WHERE path = ? ORDER BY position`, path)
	if err != nil {
		return nil, fmt.Errorf("reading items of %s: %w", path, err)
	}
	defer func() { _ = rows.Close() }()

	var out []Item
	for rows.Next() {
		var it Item
		var done int
		if err := rows.Scan(&it.Path, &it.Position, &it.ID, &it.Text, &done, &it.Minutes,
			&it.CreatedAt, &it.DoneAt, &it.From, &it.Carried, &it.Recurring, &it.Depth); err != nil {
			return nil, fmt.Errorf("scanning item: %w", err)
		}
		it.Done = done == 1
		out = append(out, it)
	}
	return out, rows.Err()
}

var ftsUnsafe = regexp.MustCompile(`[^\p{L}\p{N}\s_-]+`)

// Search runs a full-text query. The text comes straight from the user, so it is
// reduced to bare terms and each is quoted: FTS5 would otherwise treat stray
// quotes and operators as syntax and return an error mid-typing.
func (i *Index) Search(query string) ([]Hit, error) {
	terms := strings.Fields(ftsUnsafe.ReplaceAllString(query, " "))
	if len(terms) == 0 {
		return nil, nil
	}
	for n, t := range terms {
		terms[n] = `"` + t + `"`
	}

	rows, err := i.db.Query(
		`SELECT path, snippet(notes_fts, 1, '', '', '…', 12)
		 FROM notes_fts WHERE notes_fts MATCH ? ORDER BY rank LIMIT 200`,
		strings.Join(terms, " "))
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.Path, &h.Snippet); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Labels returns every label with how many places it is used, most used first.
func (i *Index) Labels() ([]Label, error) {
	rows, err := i.db.Query(`SELECT name, COUNT(*) FROM labels GROUP BY name ORDER BY COUNT(*) DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("reading labels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.Name, &l.Count); err != nil {
			return nil, fmt.Errorf("scanning label: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// NotesByLabel returns the notes carrying a label, sorted by path.
func (i *Index) NotesByLabel(name string) ([]string, error) {
	rows, err := i.db.Query(`SELECT DISTINCT path FROM labels WHERE name = ? ORDER BY path`, name)
	if err != nil {
		return nil, fmt.Errorf("reading notes for label %q: %w", name, err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning path: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MinutesBetween totals the hours worked over an inclusive date range.
func (i *Index) MinutesBetween(from, to string) (int, error) {
	var total sql.NullInt64
	err := i.db.QueryRow(
		`SELECT SUM(minutes) FROM notes WHERE type = 'workplan' AND date >= ? AND date <= ?`,
		from, to).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("totalling hours: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return int(total.Int64), nil
}

// Workplans lists the dated daily notes, newest first, with their open and done
// counts so the sidebar can show progress without opening each file.
func (i *Index) Workplans() ([]Workplan, error) {
	rows, err := i.db.Query(`
		SELECT n.path, n.date, n.hours, n.minutes, n.daytype,
		       COALESCE(SUM(CASE WHEN i.done = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN i.done = 1 THEN 1 ELSE 0 END), 0)
		FROM notes n
		LEFT JOIN items i ON i.path = n.path
		WHERE n.type = 'workplan'
		GROUP BY n.path
		ORDER BY n.date DESC`)
	if err != nil {
		return nil, fmt.Errorf("reading workplans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Workplan
	for rows.Next() {
		var w Workplan
		if err := rows.Scan(&w.Path, &w.Date, &w.Hours, &w.Minutes, &w.DayType, &w.Open, &w.Done); err != nil {
			return nil, fmt.Errorf("scanning workplan: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Snapshot renders the whole index as stable text. It exists so a test can prove
// that rebuilding from scratch produces exactly what incremental updates did.
func (i *Index) Snapshot() (string, error) {
	var b strings.Builder

	noteRows, err := i.db.Query(`SELECT path, id, type, date, hours, minutes, daytype FROM notes ORDER BY path`)
	if err != nil {
		return "", fmt.Errorf("snapshotting notes: %w", err)
	}
	defer func() { _ = noteRows.Close() }()
	for noteRows.Next() {
		var path, id, typ, date, hours, daytype string
		var minutes int
		if err := noteRows.Scan(&path, &id, &typ, &date, &hours, &minutes, &daytype); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "note %s id=%s type=%s date=%s hours=%s minutes=%d daytype=%s\n",
			path, id, typ, date, hours, minutes, daytype)
	}
	if err := noteRows.Err(); err != nil {
		return "", err
	}

	itemRows, err := i.db.Query(
		`SELECT path, position, item_id, text, done, minutes, created_at, done_at, from_date, carried, recurring, depth
		 FROM items ORDER BY path, position`)
	if err != nil {
		return "", fmt.Errorf("snapshotting items: %w", err)
	}
	defer func() { _ = itemRows.Close() }()
	for itemRows.Next() {
		var it Item
		var done int
		if err := itemRows.Scan(&it.Path, &it.Position, &it.ID, &it.Text, &done, &it.Minutes,
			&it.CreatedAt, &it.DoneAt, &it.From, &it.Carried, &it.Recurring, &it.Depth); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "item %s#%d id=%s done=%d minutes=%d t=%s done_at=%s from=%s carried=%d rec=%s depth=%d text=%s\n",
			it.Path, it.Position, it.ID, done, it.Minutes, it.CreatedAt, it.DoneAt, it.From, it.Carried, it.Recurring, it.Depth, it.Text)
	}
	if err := itemRows.Err(); err != nil {
		return "", err
	}

	labelRows, err := i.db.Query(`SELECT name, path, item_id FROM labels ORDER BY name, path, item_id`)
	if err != nil {
		return "", fmt.Errorf("snapshotting labels: %w", err)
	}
	defer func() { _ = labelRows.Close() }()
	for labelRows.Next() {
		var name, path, itemID string
		if err := labelRows.Scan(&name, &path, &itemID); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "label %s %s %s\n", name, path, itemID)
	}
	return b.String(), labelRows.Err()
}
