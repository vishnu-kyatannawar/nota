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
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/internal/update"
	"github.com/vishnu-kyatannawar/nota/services"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

// The window's own icon. Without it GTK shows the toolkit default (a "W" in a
// yellow circle) in the title bar, Alt-Tab and the taskbar; the launcher entry
// has its own copy via install.sh.
//
//go:embed build/appicon.png
var appIcon []byte

func init() {
	// The frontend listens for these; registering them gives the generated
	// bindings a typed API for each.
	application.RegisterEvent[string]("note:changed")
	application.RegisterEvent[string]("workplan:rolled")
	application.RegisterEvent[services.UpdateState]("update:state")
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

	// Sweep the binary a previous Windows update moved aside. Harmless if it
	// is not there, which is every launch on Linux.
	if exe, err := update.Target(); err == nil {
		update.CleanupOld(exe)
	}

	app := application.New(application.Options{
		Name:        "Nota",
		Description: "Daily workplans and notes, stored as plain markdown",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(services.NewAppService(core)),
			application.NewService(services.NewNotesService(core)),
			application.NewService(services.NewWorkplanService(core)),
			application.NewService(services.NewSearchService(core)),
			application.NewService(services.NewBackupService(core)),
			application.NewService(services.NewUpdateService(core)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	appService := services.NewAppService(core)
	window := app.Window.NewWithOptions(windowOptions(settings.Window, settings.Theme))
	rememberWindow(window, appService)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	core.SetUpdateEmitter(func(st services.UpdateState) { app.Event.Emit("update:state", st) })

	watchVault(ctx, app, core)
	rollAtMidnight(ctx, app, core)
	checkForUpdates(ctx, core)

	return app.Run()
}

// checkForUpdates looks for a new release shortly after launch and once a day
// after that — but only if the user has said the app may. Until that question
// is answered, and forever if the answer was no, this makes no request at all:
// it is the only outbound traffic Nota has.
func checkForUpdates(ctx context.Context, core *services.Core) {
	go func() {
		svc := services.NewUpdateService(core)
		// Let the window come up first; nothing here is urgent.
		delay := 10 * time.Second
		for {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			delay = 24 * time.Hour
			// Scheduled is a no-op, and makes no request, unless the user
			// agreed to automatic checks.
			svc.Scheduled()
		}
	}()
}

// windowOptions restores the window where the user left it. With nothing valid
// saved — a first launch, or bounds that would put the window off-screen or below
// the layout minimum — it opens maximised, so the app takes the whole screen by
// default rather than a small box in the corner.
func windowOptions(saved config.Window, theme string) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Title:            "Nota",
		MinWidth:         config.MinWindowWidth,
		MinHeight:        config.MinWindowHeight,
		BackgroundColour: backgroundFor(theme),
		URL:              "/",
	}
	if !saved.Valid() {
		opts.Width, opts.Height = 1280, 800
		opts.StartState = application.WindowStateMaximised
		return opts
	}
	opts.X, opts.Y = saved.X, saved.Y
	opts.Width, opts.Height = saved.Width, saved.Height
	if saved.Maximised {
		opts.StartState = application.WindowStateMaximised
	}
	return opts
}

// backgroundFor paints the window before the webview loads, so a light theme
// does not flash dark for a frame at startup.
func backgroundFor(theme string) application.RGBA {
	if theme == config.ThemeLight {
		return application.NewRGB(250, 250, 250)
	}
	return application.NewRGB(15, 16, 20)
}

// rememberWindow saves the bounds after a resize or move settles, and again on
// close. Saving is debounced because a drag fires dozens of events a second and
// each save is a settings file rewrite.
func rememberWindow(window *application.WebviewWindow, app *services.AppService) {
	var (
		mu    sync.Mutex
		timer *time.Timer
	)
	save := func() {
		b := window.Bounds()
		w := config.Window{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height, Maximised: window.IsMaximised()}
		if !w.Valid() {
			return
		}
		if err := app.SaveWindow(w); err != nil {
			log.Printf("saving window bounds: %v", err)
		}
	}
	debounced := func(*application.WindowEvent) {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(500*time.Millisecond, save)
	}
	window.OnWindowEvent(events.Common.WindowDidResize, debounced)
	window.OnWindowEvent(events.Common.WindowDidMove, debounced)
	window.OnWindowEvent(events.Common.WindowMaximise, debounced)
	window.OnWindowEvent(events.Common.WindowRestore, debounced)
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { save() })
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
