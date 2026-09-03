// Command nota is a desktop note application built around one dated workplan per day.
//
// Notes are plain markdown files on disk; that vault is the source of truth. The
// Go side owns every read and write, so the frontend never touches the filesystem
// and there is exactly one place a note can change.
package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/services"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Name:        "Nota",
		Description: "Daily workplans and notes, stored as plain markdown",
		Services: []application.Service{
			application.NewService(services.NewAppService(version, settings)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Nota",
		Width:            1200,
		Height:           800,
		MinWidth:         720,
		MinHeight:        480,
		BackgroundColour: application.NewRGB(20, 20, 22),
		URL:              "/",
	})

	return app.Run()
}

// loadSettings reads the settings from the default vault, falling back to the
// defaults on a first launch. The vault path itself cannot come from the settings
// file (it is where that file lives), so the default location is probed first and
// the file inside it is authoritative from then on.
func loadSettings() (config.Settings, error) {
	defaults := config.DefaultSettings()

	settings, err := config.Load(defaults.SettingsPath())
	if err != nil {
		return config.Settings{}, err
	}

	// A settings file may point the vault elsewhere; re-read from that location so
	// the vault stays self-describing.
	if settings.VaultPath != defaults.VaultPath {
		relocated, err := config.Load(settings.SettingsPath())
		if err != nil {
			return config.Settings{}, err
		}
		settings = relocated
	}

	if err := os.MkdirAll(filepath.Join(settings.WorkplanDir()), 0o755); err != nil {
		return config.Settings{}, err
	}
	return settings, nil
}
