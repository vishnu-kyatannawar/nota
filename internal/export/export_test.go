package export

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

func seed(t *testing.T) *vault.Vault {
	t.Helper()
	v, err := vault.Open(filepath.Join(t.TempDir(), "Notes"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"Workplans/2026-09-01.md":      "---\ntype: workplan\ndate: 2026-09-01\n---\n\n- [x] Done <!--n id:A1-->\n",
		"Workplans/2026-09-02.md":      "---\ntype: workplan\ndate: 2026-09-02\n---\n\n- [ ] Open <!--n id:A2-->\n",
		"Projects/rv/api.md":           "# Api\n\nSome prose.\n",
		".nota/settings.json":          `{"vaultPath":"/somewhere"}`,
		".nota/templates/recurring.md": "- [ ] Check calendar @daily\n",
	}
	for p, content := range files {
		if err := v.WriteRaw(p, content); err != nil {
			t.Fatal(err)
		}
	}
	// The index is derived and must not travel in the bundle.
	if err := os.WriteFile(filepath.Join(v.Root(), ".nota", "index.db"), []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	return v
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(abs string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExportThenRestoreReproducesTheVault(t *testing.T) {
	src := seed(t)
	before := snapshot(t, src.Root())
	delete(before, ".nota/index.db") // derived, deliberately not exported

	bundle := filepath.Join(t.TempDir(), "nota-export.zip")
	if err := Export(src, bundle); err != nil {
		t.Fatalf("Export: %v", err)
	}

	dst, err := vault.Open(filepath.Join(t.TempDir(), "Restored"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Restore(bundle, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	after := snapshot(t, dst.Root())
	if len(after) != len(before) {
		t.Fatalf("restored %d files, exported %d\nbefore: %v\nafter: %v", len(after), len(before), keys(before), keys(after))
	}
	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("%s is missing after restore", path)
			continue
		}
		if got != want {
			t.Errorf("%s differs after restore.\ngot:  %q\nwant: %q", path, got, want)
		}
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The index is rebuilt from the markdown, so shipping it would only bloat the
// bundle and risk restoring a stale cache over fresh notes.
func TestExportExcludesTheIndex(t *testing.T) {
	src := seed(t)
	bundle := filepath.Join(t.TempDir(), "nota-export.zip")
	if err := Export(src, bundle); err != nil {
		t.Fatal(err)
	}

	r, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if strings.Contains(f.Name, "index.db") {
			t.Errorf("the index was included in the bundle: %s", f.Name)
		}
	}
}

func TestExportIncludesSettingsAndTemplates(t *testing.T) {
	src := seed(t)
	bundle := filepath.Join(t.TempDir(), "nota-export.zip")
	if err := Export(src, bundle); err != nil {
		t.Fatal(err)
	}

	r, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	found := map[string]bool{}
	for _, f := range r.File {
		found[f.Name] = true
	}
	for _, want := range []string{".nota/settings.json", ".nota/templates/recurring.md", "Workplans/2026-09-01.md"} {
		if !found[want] {
			t.Errorf("%s is missing from the bundle", want)
		}
	}
}

// A bundle is a file from outside the application. An entry naming a path above
// the destination must not be allowed to write there.
func TestRestoreRejectsPathsEscapingTheVault(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "evil.zip")

	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, name := range []string{"../escaped.md", "a/../../escaped.md"} {
		e, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write([]byte("owned")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst, err := vault.Open(filepath.Join(dir, "Restored"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Restore(bundle, dst); err == nil {
		t.Error("Restore accepted an entry pointing outside the vault")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.md")); err == nil {
		t.Error("a file was written outside the vault")
	}
}

func TestRestoreOverAnExistingVaultReplacesMatchingFiles(t *testing.T) {
	src := seed(t)
	bundle := filepath.Join(t.TempDir(), "nota-export.zip")
	if err := Export(src, bundle); err != nil {
		t.Fatal(err)
	}

	dst, err := vault.Open(filepath.Join(t.TempDir(), "Restored"))
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.WriteRaw("Workplans/2026-09-01.md", "stale content\n"); err != nil {
		t.Fatal(err)
	}

	if err := Restore(bundle, dst); err != nil {
		t.Fatal(err)
	}
	got, err := dst.ReadRaw("Workplans/2026-09-01.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "stale") {
		t.Error("restore left the stale file in place")
	}
}

func TestDefaultBundleName(t *testing.T) {
	if got := BundleName("2026-09-02"); got != "nota-export-2026-09-02.zip" {
		t.Errorf("BundleName() = %q", got)
	}
}

func TestExportCarriesPastedImages(t *testing.T) {
	v := seed(t)
	rel, err := v.SaveAttachment(".png", []byte("\x89PNG body"))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "bundle.zip")
	if err := Export(v, dest); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name == rel {
			return
		}
	}
	// A backup without the images is a backup that loses the notes' pictures.
	t.Errorf("%s is missing from the bundle", rel)
}
