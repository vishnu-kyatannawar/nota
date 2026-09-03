package services

import (
	"fmt"

	"github.com/vishnu-kyatannawar/nota/internal/config"
)

// AppService exposes application-level state to the frontend.
type AppService struct {
	core *Core
}

// NewAppService returns the service bound as AppService.
func NewAppService(core *Core) *AppService { return &AppService{core: core} }

// Links every About screen points at.
const (
	RepositoryURL = "https://github.com/vishnu-kyatannawar/nota"
	WebsiteURL    = "https://vishnu-kyatannawar.github.io/nota/"
	ReleasesURL   = RepositoryURL + "/releases"
	LicenceURL    = RepositoryURL + "/blob/main/LICENSE"
)

// Info is the snapshot the frontend needs on startup.
type Info struct {
	Version     string       `json:"version"`
	VaultPath   string       `json:"vaultPath"`
	WorkplanDir string       `json:"workplanDir"`
	Theme       string       `json:"theme"`
	Fonts       config.Fonts `json:"fonts"`
	Repository  string       `json:"repository"`
	Website     string       `json:"website"`
	Releases    string       `json:"releases"`
	Licence     string       `json:"licence"`
}

// GetInfo returns the current application info.
func (a *AppService) GetInfo() Info {
	s := a.core.Settings()
	return Info{
		Version:     a.core.version,
		VaultPath:   s.VaultPath,
		WorkplanDir: s.WorkplanDir(),
		Theme:       s.Theme,
		Fonts:       s.Fonts,
		Repository:  RepositoryURL,
		Website:     WebsiteURL,
		Releases:    ReleasesURL,
		Licence:     LicenceURL,
	}
}

// SetTheme persists the theme choice: system, light or dark.
func (a *AppService) SetTheme(theme string) error {
	switch theme {
	case config.ThemeSystem, config.ThemeLight, config.ThemeDark:
	default:
		return fmt.Errorf("unknown theme %q", theme)
	}
	return a.core.updateSettings(func(s *config.Settings) { s.Theme = theme })
}

// SetFonts persists the chosen typography. Unknown ids fall back to the defaults
// on the next load, so a stale value can never leave the interface fontless.
func (a *AppService) SetFonts(f config.Fonts) error {
	return a.core.updateSettings(func(s *config.Settings) { s.Fonts = f })
}

// SaveWindow remembers the window's bounds so the next launch reopens the same
// way. Called from the window event hooks in main, not from the frontend.
func (a *AppService) SaveWindow(w config.Window) error {
	return a.core.updateSettings(func(s *config.Settings) { s.Window = w })
}

// GetSettings returns the active settings.
func (a *AppService) GetSettings() config.Settings { return a.core.Settings() }

// SaveSettings persists new settings. Changing the vault path takes effect on
// the next launch rather than mid-session, since the open vault, index and
// watcher are all bound to the current one.
func (a *AppService) SaveSettings(s config.Settings) error {
	s.VaultPath = config.ExpandHome(s.VaultPath)
	if s.VaultPath == "" {
		return fmt.Errorf("vault path must not be empty")
	}
	if s.WorkplanFolder == "" {
		s.WorkplanFolder = config.DefaultWorkplanFolder
	}

	a.core.mu.Lock()
	defer a.core.mu.Unlock()
	if err := config.Save(s.SettingsPath(), s); err != nil {
		return err
	}
	a.core.settings = s
	return nil
}
