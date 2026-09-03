package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
	t.Setenv("HOME", home)

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
	t.Setenv("HOME", home)

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

	want := Settings{
		VaultPath:        filepath.Join(dir, "MyNotes"),
		WorkplanFolder:   "Daily",
		CreateOnWeekends: false,
	}
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
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
}
