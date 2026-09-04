// Package config resolves where Nota keeps its data and loads the user's settings.
//
// The vault (a directory of markdown files) is the source of truth for notes, so
// everything here is about locating it. Settings themselves live inside the vault
// at .nota/settings.json, which keeps a vault self-contained and portable: copy
// the directory and the configuration travels with it.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultWorkplanFolder is the reserved folder holding one dated note per day.
const DefaultWorkplanFolder = "Workplans"

// Theme values. "system" follows the operating system's light/dark preference.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Fonts are the faces the interface, the notes area and code use, by id, plus a
// size step. The ids name faces bundled with the application (see the frontend
// catalogue), or "system" for the platform default.
type Fonts struct {
	UI    string `json:"ui"`
	Notes string `json:"notes"`
	Code  string `json:"code"`
	Size  string `json:"size"`
}

// How the items and notes sections sit together when a page shows both.
const (
	// SplitRows stacks them, notes under items.
	SplitRows = "rows"
	// SplitColumns puts them side by side.
	SplitColumns = "columns"
)

// normalisedSplit puts an unrecognised arrangement back to stacked.
func normalisedSplit(split string) string {
	if split == SplitColumns {
		return SplitColumns
	}
	return SplitRows
}

// What the update check may be set to.
const (
	// UpdatesAsk is the state before the user has answered: the app has not
	// contacted the network and will not until it is told to.
	UpdatesAsk = "ask"
	// UpdatesAuto checks on launch and once a day.
	UpdatesAuto = "auto"
	// UpdatesNever checks only when the user presses Check now.
	UpdatesNever = "never"
)

// Updates controls whether the app looks for new releases. Nota makes no
// network requests of any other kind, so this single field is the whole of its
// outbound traffic — which is why it starts at "ask" rather than at a default
// somebody has to discover and turn off.
type Updates struct {
	// Check is "ask" until the user answers, then "auto" or "never".
	Check string `json:"check"`
	// LastCheck is when the last successful check ran, RFC 3339, so relaunching
	// repeatedly does not re-ask GitHub each time. Empty until the first one.
	LastCheck string `json:"lastCheck,omitempty"`
}

// DefaultUpdates leaves the choice to the user.
func DefaultUpdates() Updates {
	return Updates{Check: UpdatesAsk}
}

// normalised returns the settings with an unrecognised choice put back to
// "ask", so a hand-edited or future value asks rather than assuming consent.
func (u Updates) normalised() Updates {
	switch u.Check {
	case UpdatesAsk, UpdatesAuto, UpdatesNever:
	default:
		u.Check = UpdatesAsk
	}
	return u
}

// Which ids each slot accepts. Kept in one place so a hand-edited settings file
// cannot leave the interface with a face that was never bundled.
var (
	uiFonts    = map[string]bool{"system": true, "inter": true, "manrope": true, "ibm-plex-sans": true}
	notesFonts = map[string]bool{"system": true, "inter": true, "lora": true, "source-serif-4": true}
	codeFonts  = map[string]bool{"system": true, "jetbrains-mono": true}
	fontSizes  = map[string]bool{"s": true, "m": true, "l": true}
)

// DefaultFonts is the first-launch typography.
func DefaultFonts() Fonts {
	return Fonts{UI: "inter", Notes: "inter", Code: "jetbrains-mono", Size: "m"}
}

// normalised returns the fonts with every unknown value replaced by its default.
func (f Fonts) normalised() Fonts {
	d := DefaultFonts()
	if !uiFonts[f.UI] {
		f.UI = d.UI
	}
	if !notesFonts[f.Notes] {
		f.Notes = d.Notes
	}
	if !codeFonts[f.Code] {
		f.Code = d.Code
	}
	if !fontSizes[f.Size] {
		f.Size = d.Size
	}
	return f
}

// The smallest window the layout works at; also the floor for a saved window.
const (
	MinWindowWidth  = 720
	MinWindowHeight = 480
)

// Window is the last size and position the user left the window at, so it can
// reopen the same way. The zero value means "never saved", which is how a first
// launch is told apart from a remembered one.
type Window struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

// Valid reports whether the saved window is worth restoring. A window that is
// smaller than the layout minimum, or that was dragged far enough off-screen that
// restoring it would hide it, is treated as never saved and the launch falls back
// to a maximised window.
func (w Window) Valid() bool {
	if w.Width < MinWindowWidth || w.Height < MinWindowHeight {
		return false
	}
	// Multi-monitor setups legitimately place windows at negative coordinates,
	// so only reject positions no display could plausibly contain.
	const farthest = 16384
	if w.X < -farthest || w.X > farthest || w.Y < -farthest || w.Y > farthest {
		return false
	}
	return true
}

// Settings is the user's configuration, persisted as .nota/settings.json.
type Settings struct {
	// VaultPath is the root directory of the markdown vault, always absolute.
	VaultPath string `json:"vaultPath"`
	// WorkplanFolder is the vault-relative folder holding the dated daily notes.
	WorkplanFolder string `json:"workplanFolder"`
	// CreateOnWeekends controls whether a workplan is created on Saturday and
	// Sunday. Rollover copes with gaps either way; this only decides whether a
	// weekend gets a note at all.
	CreateOnWeekends bool `json:"createOnWeekends"`
	// Theme is "system", "light" or "dark".
	Theme string `json:"theme"`
	// Window remembers where the window was left; see Window.Valid.
	Window Window `json:"window"`
	// Fonts is the chosen typography; see Fonts.
	Fonts Fonts `json:"fonts"`
	// Updates controls the release check; see Updates.
	Updates Updates `json:"updates"`
	// Split is how the items and notes sections sit together on a page that
	// shows both: "rows" stacks them, "columns" puts them side by side.
	Split string `json:"split"`
}

// DefaultSettings returns the configuration used on a first launch.
func DefaultSettings() Settings {
	return Settings{
		VaultPath:        ExpandHome(filepath.Join("~", "Notes")),
		WorkplanFolder:   DefaultWorkplanFolder,
		CreateOnWeekends: true,
		Theme:            ThemeSystem,
		Fonts:            DefaultFonts(),
		Updates:          DefaultUpdates(),
		Split:            SplitRows,
	}
}

// ExpandHome resolves a leading ~ to the user's home directory. A bare path, an
// absolute path, and the ~user form are all returned unchanged — only the plain
// "~" and "~/..." forms are expanded, which is what settings files realistically
// contain.
func ExpandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~"+string(os.PathSeparator)) && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

// AppDir is the vault-internal directory holding settings, templates and the index.
func (s Settings) AppDir() string {
	return filepath.Join(s.VaultPath, ".nota")
}

// SettingsPath is where this configuration is persisted.
func (s Settings) SettingsPath() string {
	return filepath.Join(s.AppDir(), "settings.json")
}

// IndexPath is where the rebuildable SQLite cache lives.
func (s Settings) IndexPath() string {
	return filepath.Join(s.AppDir(), "index.db")
}

// TemplatesDir holds the recurring item templates.
func (s Settings) TemplatesDir() string {
	return filepath.Join(s.AppDir(), "templates")
}

// WorkplanDir is the absolute path of the reserved workplan folder.
func (s Settings) WorkplanDir() string {
	folder := s.WorkplanFolder
	if folder == "" {
		folder = DefaultWorkplanFolder
	}
	return filepath.Join(s.VaultPath, folder)
}

// Load reads settings from path. A missing file is not an error: it means a first
// launch, so the defaults are returned. Fields absent from the file fall back to
// their defaults rather than landing as zero values, so adding a field in a later
// version does not silently disable it for existing users.
func Load(path string) (Settings, error) {
	s := DefaultSettings()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return Settings{}, fmt.Errorf("reading settings %s: %w", path, err)
	}

	// Unmarshalling into the defaults leaves absent fields at their default value.
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("parsing settings %s: %w", path, err)
	}

	s.VaultPath = ExpandHome(s.VaultPath)
	switch s.Theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		// A hand-edited or future value must not leave the UI without a theme.
		s.Theme = ThemeSystem
	}
	s.Fonts = s.Fonts.normalised()
	s.Updates = s.Updates.normalised()
	s.Split = normalisedSplit(s.Split)
	return s, nil
}

// Save writes settings to path as indented JSON, creating parent directories as
// needed. The write goes to a temporary file first and is then renamed, so an
// interrupted save cannot leave a half-written settings file behind.
func Save(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return fmt.Errorf("creating temporary settings file: %w", err)
	}
	tmpName := tmp.Name()
	// Best effort: on the success path the rename has already consumed it.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		// Already failing; the close error would only mask the useful one.
		_ = tmp.Close()
		return fmt.Errorf("writing settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing settings: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("setting settings permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("installing settings: %w", err)
	}
	return nil
}
