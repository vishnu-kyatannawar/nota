// Package services holds the types bound into the frontend by Wails.
//
// Everything here is deliberately thin: a service validates its inputs, calls
// into internal/..., and returns. Keeping the logic out of this layer means the
// Wails v3 beta API can churn without dragging application behaviour with it,
// and it keeps internal/ testable without a running webview.
package services

import (
	"fmt"

	"github.com/vishnu-kyatannawar/nota/internal/config"
)

// AppService exposes application-level state to the frontend.
type AppService struct {
	version  string
	settings config.Settings
}

// NewAppService returns a service seeded with the loaded settings.
func NewAppService(version string, settings config.Settings) *AppService {
	return &AppService{version: version, settings: settings}
}

// Info is the snapshot the frontend needs on startup.
type Info struct {
	Version     string `json:"version"`
	VaultPath   string `json:"vaultPath"`
	WorkplanDir string `json:"workplanDir"`
}

// GetInfo returns the current application info.
func (a *AppService) GetInfo() Info {
	return Info{
		Version:     a.version,
		VaultPath:   a.settings.VaultPath,
		WorkplanDir: a.settings.WorkplanDir(),
	}
}

// GetSettings returns the active settings.
func (a *AppService) GetSettings() config.Settings {
	return a.settings
}

// SaveSettings persists new settings and adopts them for this session.
func (a *AppService) SaveSettings(s config.Settings) error {
	s.VaultPath = config.ExpandHome(s.VaultPath)
	if s.VaultPath == "" {
		return fmt.Errorf("vault path must not be empty")
	}
	if err := config.Save(s.SettingsPath(), s); err != nil {
		return err
	}
	a.settings = s
	return nil
}
