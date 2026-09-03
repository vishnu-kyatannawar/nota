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

// Info is the snapshot the frontend needs on startup.
type Info struct {
	Version     string `json:"version"`
	VaultPath   string `json:"vaultPath"`
	WorkplanDir string `json:"workplanDir"`
}

// GetInfo returns the current application info.
func (a *AppService) GetInfo() Info {
	s := a.core.Settings()
	return Info{
		Version:     a.core.version,
		VaultPath:   s.VaultPath,
		WorkplanDir: s.WorkplanDir(),
	}
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
