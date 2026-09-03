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
}

// DefaultSettings returns the configuration used on a first launch.
func DefaultSettings() Settings {
	return Settings{
		VaultPath:        ExpandHome(filepath.Join("~", "Notes")),
		WorkplanFolder:   DefaultWorkplanFolder,
		CreateOnWeekends: true,
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
