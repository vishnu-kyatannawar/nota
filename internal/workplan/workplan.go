// Package workplan owns the dated daily notes.
//
// One note per day lives in a reserved folder, named for its date. When a new
// day starts, unfinished items roll forward from the most recent previous day
// and completed ones are left behind on the day they were completed — that
// carry-forward is the behaviour the whole application exists for.
//
// Everything here is idempotent. Ensure runs on launch, on a midnight timer, and
// whenever the window regains focus, so running it repeatedly must never
// duplicate an item or disturb a day the user has already edited.
package workplan

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

// DateLayout is how a workplan note is named and dated.
const DateLayout = "2006-01-02"

// TemplatePath is the vault-relative file holding the recurring items.
const TemplatePath = ".nota/templates/recurring.md"

// Day types. Anything other than work pins the day's hours at zero by default,
// though the user may still log time against it.
const (
	DayWork    = "work"
	DayWeekend = "weekend"
	DayLeave   = "leave"
	DayHoliday = "holiday"
)

var validDayTypes = map[string]bool{
	DayWork: true, DayWeekend: true, DayLeave: true, DayHoliday: true,
}

// ErrInvalidHours is returned for a duration that is not "hh:mm".
var ErrInvalidHours = errors.New("hours must be in hh:mm form, for example 07:30")

// ErrInvalidDayType is returned for a day type outside the known set.
var ErrInvalidDayType = errors.New("unknown day type")

// Options configures the manager.
type Options struct {
	// Folder is the vault-relative reserved folder for daily notes.
	Folder string
	// CreateOnWeekends decides whether Saturday and Sunday get a note at all.
	CreateOnWeekends bool
	// NewID mints ids for seeded items. Injectable so tests can pin them down;
	// defaults to a ULID, which sorts by creation time.
	NewID func() string
}

// Manager creates and rolls over the daily notes.
type Manager struct {
	vault *vault.Vault
	opts  Options
}

// New returns a manager over a vault.
func New(v *vault.Vault, opts Options) *Manager {
	if opts.Folder == "" {
		opts.Folder = "Workplans"
	}
	if opts.NewID == nil {
		opts.NewID = func() string {
			return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
		}
	}
	return &Manager{vault: v, opts: opts}
}

// PathFor is the vault-relative path of a given day's note.
func (m *Manager) PathFor(day time.Time) string {
	return m.opts.Folder + "/" + day.Format(DateLayout) + mdnote.Ext
}

// Ensure makes sure the note for day exists, creating it by rolling the previous
// workplan forward and seeding any recurring items that are due.
//
// It returns the note's path, or an empty path when the day is a weekend and
// weekend notes are switched off. An existing note is never touched, so a day the
// user has already worked on is safe from every later call.
func (m *Manager) Ensure(day time.Time) (string, error) {
	if !m.opts.CreateOnWeekends && isWeekend(day) {
		return "", nil
	}

	path := m.PathFor(day)
	if _, err := m.vault.ReadNote(path); err == nil {
		return path, nil
	}

	note := &mdnote.Note{
		Type:           "workplan",
		Date:           day.Format(DateLayout),
		Hours:          mdnote.FormatDuration(0),
		DayType:        dayTypeFor(day),
		HadFrontmatter: true,
	}

	carried, err := m.carryForward(day)
	if err != nil {
		return "", err
	}
	note.Items = carried

	seeded, err := m.seedRecurring(day, note.Items)
	if err != nil {
		return "", err
	}
	note.Items = append(note.Items, seeded...)

	if err := m.vault.WriteNote(path, note); err != nil {
		return "", err
	}
	return path, nil
}

// carryForward reads the most recent earlier workplan and returns its unfinished
// items, ready to be written into the new day.
func (m *Manager) carryForward(day time.Time) ([]mdnote.Item, error) {
	prevPath, prevDate, err := m.previousWorkplan(day)
	if err != nil || prevPath == "" {
		return nil, err
	}

	prev, err := m.vault.ReadNote(prevPath)
	if err != nil {
		return nil, err
	}

	var out []mdnote.Item
	for _, it := range prev.Items {
		if it.Done {
			// Completed work stays on the day it was completed.
			continue
		}
		// The item keeps its identity, its original creation time and anything
		// logged against it; only the completion state and carry counters move.
		it.DoneAt = ""
		if it.From == "" {
			it.From = prevDate
		}
		it.Carried++
		out = append(out, it)
	}
	return out, nil
}

// previousWorkplan finds the latest dated note before day. It deliberately looks
// for the most recent workplan rather than literally yesterday, so weekends,
// leave and holidays do not break the chain.
func (m *Manager) previousWorkplan(day time.Time) (path, date string, err error) {
	paths, err := m.vault.ListNotes()
	if err != nil {
		return "", "", err
	}
	prefix := m.opts.Folder + "/"
	today := day.Format(DateLayout)

	best := ""
	for _, p := range paths {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(p, prefix), mdnote.Ext)
		if _, perr := time.Parse(DateLayout, name); perr != nil {
			continue
		}
		if name >= today {
			continue
		}
		if name > best {
			best, path = name, p
		}
	}
	return path, best, nil
}

// seedRecurring returns the recurring items due on day that are not already
// present. Matching on the recurring id means an item carried over from
// yesterday suppresses today's seed, and one the user deleted for today stays
// deleted for today.
func (m *Manager) seedRecurring(day time.Time, existing []mdnote.Item) ([]mdnote.Item, error) {
	templates, err := m.Templates()
	if err != nil {
		return nil, err
	}

	present := map[string]bool{}
	for _, it := range existing {
		if it.Recurring != "" {
			present[it.Recurring] = true
		}
	}

	// Seeded items are stamped with the clock time Ensure ran at, which for the
	// usual midnight rollover is the start of the day.
	stamp := day.Format("15:04")

	var out []mdnote.Item
	for _, tpl := range templates {
		if present[tpl.ID] || !tpl.DueOn(day) {
			continue
		}
		out = append(out, mdnote.Item{
			// A seeded item needs its own id like any other: without one the
			// editor cannot address it to tick, retext or delete it.
			ID:        m.opts.NewID(),
			Text:      tpl.Text,
			CreatedAt: stamp,
			Recurring: tpl.ID,
		})
	}
	return out, nil
}

// Template is one recurring item and how often it appears.
type Template struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Cadence string `json:"cadence"`
}

var cadenceToken = regexp.MustCompile(`\s*@(daily|weekdays|weekly:(?:mon|tue|wed|thu|fri|sat|sun))\b`)

var weekdayNames = map[string]time.Weekday{
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday, "sun": time.Sunday,
}

// DueOn reports whether the template should appear on a given day.
func (t Template) DueOn(day time.Time) bool {
	switch {
	case t.Cadence == "daily":
		return true
	case t.Cadence == "weekdays":
		return !isWeekend(day)
	case strings.HasPrefix(t.Cadence, "weekly:"):
		want, ok := weekdayNames[strings.TrimPrefix(t.Cadence, "weekly:")]
		return ok && day.Weekday() == want
	default:
		return false
	}
}

// Templates reads the recurring items file. A missing file simply means none are
// configured. Each template needs a stable id so seeding can tell whether today
// already has it; one is derived from the text and written back on first read, so
// the file stays hand-editable without the user having to invent ids.
func (m *Manager) Templates() ([]Template, error) {
	raw, err := m.vault.ReadRaw(TemplatePath)
	if errors.Is(err, fs.ErrNotExist) {
		// No template file simply means no recurring items are configured.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading recurring templates: %w", err)
	}

	note, err := mdnote.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing recurring templates: %w", err)
	}

	var (
		out     []Template
		changed bool
	)
	for i := range note.Items {
		it := &note.Items[i]

		cadence := "daily"
		if m := cadenceToken.FindStringSubmatch(it.Text); m != nil {
			cadence = m[1]
		}
		text := strings.TrimSpace(cadenceToken.ReplaceAllString(it.Text, ""))
		if text == "" {
			continue
		}

		if it.Recurring == "" {
			it.Recurring = slug(text)
			changed = true
		}
		out = append(out, Template{ID: it.Recurring, Text: text, Cadence: cadence})
	}

	if changed {
		// Best effort: the ids are a convenience, and a read-only vault should
		// not stop recurring items from working.
		_ = m.vault.WriteNote(TemplatePath, note)
	}
	return out, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	out := nonSlug.ReplaceAllString(strings.ToLower(s), "-")
	out = strings.Trim(out, "-")
	if len(out) > 40 {
		out = out[:40]
	}
	if out == "" {
		out = "item"
	}
	return out
}

// SetHours records the hours worked on a day.
func (m *Manager) SetHours(path, hours string) error {
	if _, ok := mdnote.ParseDuration(hours); !ok {
		return fmt.Errorf("%w: got %q", ErrInvalidHours, hours)
	}
	return m.mutate(path, func(n *mdnote.Note) { n.Hours = hours })
}

// SetDayType marks a day as work, weekend, leave or holiday. Marking it pins the
// default at zero hours but does not erase time already logged, since a person
// may genuinely have worked on a day off.
func (m *Manager) SetDayType(path, dayType string) error {
	if !validDayTypes[dayType] {
		return fmt.Errorf("%w: %q", ErrInvalidDayType, dayType)
	}
	return m.mutate(path, func(n *mdnote.Note) { n.DayType = dayType })
}

// SuggestedMinutes totals the time logged against a day's items. It is only ever
// a suggestion to pre-fill the field: plenty of a working day — meetings, calls,
// helping someone — never lands on an action item, so the day total stays a value
// the user owns.
func (m *Manager) SuggestedMinutes(path string) (int, error) {
	note, err := m.vault.ReadNote(path)
	if err != nil {
		return 0, err
	}
	return note.TotalMinutes(), nil
}

func (m *Manager) mutate(path string, apply func(*mdnote.Note)) error {
	note, err := m.vault.ReadNote(path)
	if err != nil {
		return err
	}
	apply(note)
	return m.vault.WriteNote(path, note)
}

func isWeekend(day time.Time) bool {
	switch day.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	default:
		return false
	}
}

func dayTypeFor(day time.Time) string {
	if isWeekend(day) {
		return DayWeekend
	}
	return DayWork
}
