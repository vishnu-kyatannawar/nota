package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setHome points os.UserHomeDir at dir for the duration of the test. Go reads
// USERPROFILE on Windows and HOME everywhere else, so the test has to set the
// one the platform actually consults rather than assuming HOME.
func setHome(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		return
	}
	t.Setenv("HOME", dir)
}

func TestDefaultSettings(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	got := DefaultSettings()

	if want := filepath.Join(home, "Notes"); got.VaultPath != want {
		t.Errorf("VaultPath = %q, want %q", got.VaultPath, want)
	}
	if got.WorkplanFolder != "Workplans" {
		t.Errorf("WorkplanFolder = %q, want %q", got.WorkplanFolder, "Workplans")
	}
	if !got.CreateOnWeekends {
		t.Error("CreateOnWeekends = false, want true (default is every day)")
	}
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tilde alone", "~", home},
		{"tilde slash", "~/Notes", filepath.Join(home, "Notes")},
		{"nested", "~/a/b/c", filepath.Join(home, "a", "b", "c")},
		{"absolute untouched", "/srv/notes", "/srv/notes"},
		{"empty untouched", "", ""},
		{"tilde mid-path untouched", "/x/~/y", "/x/~/y"},
		{"tilde user unsupported", "~bob/Notes", "~bob/Notes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExpandHome(tt.in); got != tt.want {
				t.Errorf("ExpandHome(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	got, err := Load(filepath.Join(home, "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if got.VaultPath != DefaultSettings().VaultPath {
		t.Errorf("VaultPath = %q, want the default %q", got.VaultPath, DefaultSettings().VaultPath)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "settings.json")

	want := DefaultSettings()
	want.VaultPath = filepath.Join(dir, "MyNotes")
	want.WorkplanFolder = "Daily"
	want.CreateOnWeekends = false
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestLoadExpandsTildeInStoredPath(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	path := filepath.Join(home, "settings.json")

	if err := os.WriteFile(path, []byte(`{"vaultPath":"~/Elsewhere"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := filepath.Join(home, "Elsewhere"); got.VaultPath != want {
		t.Errorf("VaultPath = %q, want %q", got.VaultPath, want)
	}
}

func TestLoadFillsMissingFieldsFromDefaults(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	path := filepath.Join(home, "settings.json")

	// Only vaultPath is set; the rest must fall back rather than land as zero values.
	if err := os.WriteFile(path, []byte(`{"vaultPath":"/srv/notes"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.WorkplanFolder != "Workplans" {
		t.Errorf("WorkplanFolder = %q, want the default %q", got.WorkplanFolder, "Workplans")
	}
	if !got.CreateOnWeekends {
		t.Error("CreateOnWeekends = false, want the default true")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Error("Load on malformed JSON returned nil error, want an error")
	}
}

func TestAppDirIsInsideVault(t *testing.T) {
	s := Settings{VaultPath: "/srv/notes"}
	if got, want := s.AppDir(), filepath.Join("/srv/notes", ".nota"); got != want {
		t.Errorf("AppDir() = %q, want %q", got, want)
	}
	if got, want := s.SettingsPath(), filepath.Join("/srv/notes", ".nota", "settings.json"); got != want {
		t.Errorf("SettingsPath() = %q, want %q", got, want)
	}
	if got, want := s.WorkplanDir(), filepath.Join("/srv/notes", "Workplans"); got != want {
		t.Errorf("WorkplanDir() = %q, want %q", got, want)
	}
	if got, want := s.IndexPath(), filepath.Join("/srv/notes", ".nota", "index.db"); got != want {
		t.Errorf("IndexPath() = %q, want %q", got, want)
	}
	if got, want := s.TemplatesDir(), filepath.Join("/srv/notes", ".nota", "templates"); got != want {
		t.Errorf("TemplatesDir() = %q, want %q", got, want)
	}
}

func TestDefaultThemeIsSystem(t *testing.T) {
	setHome(t, t.TempDir())
	if got := DefaultSettings().Theme; got != ThemeSystem {
		t.Errorf("Theme = %q, want %q", got, ThemeSystem)
	}
}

func TestLoadFillsThemeWhenAbsentOrUnknown(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	path := filepath.Join(home, "settings.json")

	tests := []struct {
		name string
		json string
		want string
	}{
		{"absent falls back", `{"vaultPath":"/srv/notes"}`, ThemeSystem},
		{"light kept", `{"vaultPath":"/srv/notes","theme":"light"}`, ThemeLight},
		{"dark kept", `{"vaultPath":"/srv/notes","theme":"dark"}`, ThemeDark},
		{"unknown value falls back", `{"vaultPath":"/srv/notes","theme":"neon"}`, ThemeSystem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.json), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if got.Theme != tt.want {
				t.Errorf("Theme = %q, want %q", got.Theme, tt.want)
			}
		})
	}
}

func TestWindowValid(t *testing.T) {
	tests := []struct {
		name string
		w    Window
		want bool
	}{
		{"zero value is not a saved window", Window{}, false},
		{"sane bounds", Window{X: 100, Y: 80, Width: 1280, Height: 800}, true},
		{"at the minimum", Window{Width: MinWindowWidth, Height: MinWindowHeight}, true},
		{"below minimum width", Window{Width: MinWindowWidth - 1, Height: 800}, false},
		{"below minimum height", Window{Width: 1280, Height: MinWindowHeight - 1}, false},
		{"negative size", Window{Width: -10, Height: -10}, false},
		{"maximised with sane bounds", Window{Width: 1280, Height: 800, Maximised: true}, true},
		{"absurdly large is still valid, the OS clamps it", Window{Width: 20000, Height: 20000}, true},
		// A window dragged off-screen must not be restored off-screen.
		{"far negative position", Window{X: -50000, Y: 0, Width: 1280, Height: 800}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v for %+v", got, tt.want, tt.w)
			}
		})
	}
}

func TestWindowRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	want := DefaultSettings()
	want.VaultPath = filepath.Join(dir, "Notes")
	want.Theme = ThemeDark
	want.Window = Window{X: 40, Y: 60, Width: 1440, Height: 900, Maximised: true}

	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Window != want.Window {
		t.Errorf("Window = %+v, want %+v", got.Window, want.Window)
	}
	if got.Theme != ThemeDark {
		t.Errorf("Theme = %q, want dark", got.Theme)
	}
}

// A settings file from v1 has no window or theme keys at all; it must load
// with the new defaults rather than fail or produce an invalid window.
func TestLoadV1SettingsFile(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	path := filepath.Join(home, "settings.json")
	v1 := `{"vaultPath":"/srv/notes","workplanFolder":"Workplans","createOnWeekends":true}`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != ThemeSystem {
		t.Errorf("Theme = %q, want system", got.Theme)
	}
	if got.Window.Valid() {
		t.Errorf("a v1 file must not yield a valid saved window, got %+v", got.Window)
	}
}

func TestDefaultFonts(t *testing.T) {
	setHome(t, t.TempDir())
	got := DefaultSettings().Fonts
	want := Fonts{UI: "inter", Notes: "inter", Code: "jetbrains-mono", Size: "m"}
	if got != want {
		t.Errorf("Fonts = %+v, want %+v", got, want)
	}
}

func TestLoadNormalisesUnknownFonts(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	path := filepath.Join(home, "settings.json")
	if err := os.WriteFile(path, []byte(`{"vaultPath":"/srv/notes","fonts":{"ui":"comic-sans","notes":"lora","code":"","size":"xl"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Fonts{UI: "inter", Notes: "lora", Code: "jetbrains-mono", Size: "m"}
	if got.Fonts != want {
		t.Errorf("Fonts = %+v, want %+v", got.Fonts, want)
	}
}

func TestFontsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	want := DefaultSettings()
	want.VaultPath = filepath.Join(dir, "Notes")
	want.Fonts = Fonts{UI: "manrope", Notes: "source-serif-4", Code: "system", Size: "l"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fonts != want.Fonts {
		t.Errorf("Fonts = %+v, want %+v", got.Fonts, want.Fonts)
	}
}
