// Command nota is a desktop note application built around one dated workplan per day.
//
// Notes are plain markdown files on disk; that vault is the source of truth. The
// Go side owns every read and write, so the frontend never touches the filesystem
// and there is exactly one place a note can change.
package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/services"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// The frontend listens for these; registering them gives the generated
	// bindings a typed API for each.
	application.RegisterEvent[string]("note:changed")
	application.RegisterEvent[string]("workplan:rolled")
}

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

	core, err := services.NewCore(version, settings)
	if err != nil {
		return err
	}
	defer func() { _ = core.Close() }()

	app := application.New(application.Options{
		Name:        "Nota",
		Description: "Daily workplans and notes, stored as plain markdown",
		Services: []application.Service{
			application.NewService(services.NewAppService(core)),
			application.NewService(services.NewNotesService(core)),
			application.NewService(services.NewWorkplanService(core)),
			application.NewService(services.NewSearchService(core)),
			application.NewService(services.NewBackupService(core)),
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchVault(ctx, app, core)
	rollAtMidnight(ctx, app, core)

	return app.Run()
}

// watchVault tells the frontend when a note changes underneath it, so editing a
// note in vim while Nota is open does not leave the window showing stale text.
func watchVault(ctx context.Context, app *application.App, core *services.Core) {
	changes, err := core.Vault().Watch(ctx)
	if err != nil {
		// A missing watcher costs live reload, not correctness; the application
		// is still perfectly usable without it.
		log.Printf("vault watch unavailable: %v", err)
		return
	}
	go func() {
		for change := range changes {
			if change.Op == "removed" {
				_ = core.Index().Remove(change.Path)
			} else if note, err := core.Vault().ReadNote(change.Path); err == nil {
				_ = core.Index().Update(change.Path, note)
			}
			app.Event.Emit("note:changed", change.Path)
		}
	}()
}

// rollAtMidnight creates the new day's workplan while the application is left
// running. Ensure is idempotent, so the launch-time call and this one cannot
// produce a duplicate between them.
func rollAtMidnight(ctx context.Context, app *application.App, core *services.Core) {
	go func() {
		for {
			now := time.Now()
			midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
			timer := time.NewTimer(time.Until(midnight) + time.Second)

			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				path, err := core.Plans().Ensure(time.Now())
				if err != nil {
					log.Printf("midnight rollover failed: %v", err)
					continue
				}
				if path == "" {
					continue
				}
				if note, err := core.Vault().ReadNote(path); err == nil {
					_ = core.Index().Update(path, note)
				}
				app.Event.Emit("workplan:rolled", path)
			}
		}
	}()
}

// loadSettings reads the settings from the default vault, falling back to the
// defaults on a first launch. The vault path cannot itself come from the settings
// file, since that file lives inside the vault; so the default location is probed
// first, and a settings file found there may redirect to the real vault.
func loadSettings() (config.Settings, error) {
	defaults := config.DefaultSettings()

	settings, err := config.Load(defaults.SettingsPath())
	if err != nil {
		return config.Settings{}, err
	}
	if settings.VaultPath != defaults.VaultPath {
		relocated, err := config.Load(settings.SettingsPath())
		if err != nil {
			return config.Settings{}, err
		}
		settings = relocated
	}
	return settings, nil
}
