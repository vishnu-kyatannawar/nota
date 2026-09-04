package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAttachmentStoresAnImageAndReturnsItsVaultPath(t *testing.T) {
	v := newVault(t)
	rel, err := v.SaveAttachment(".png", []byte("\x89PNG fake"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rel, AttachmentDir+"/") || !strings.HasSuffix(rel, ".png") {
		t.Errorf("path = %q, want one under %s ending in .png", rel, AttachmentDir)
	}
	// The note's markdown refers to this path, so it must resolve from the root.
	if _, err := os.Stat(filepath.Join(v.Root(), filepath.FromSlash(rel))); err != nil {
		t.Errorf("the file is not where the note would look for it: %v", err)
	}
	got, err := v.ReadAttachment(rel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\x89PNG fake" {
		t.Errorf("read back %q", got)
	}
}

func TestSaveAttachmentNamesEachOneSeparately(t *testing.T) {
	v := newVault(t)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		rel, err := v.SaveAttachment(".png", []byte("x"))
		if err != nil {
			t.Fatal(err)
		}
		if seen[rel] {
			t.Fatalf("%s was handed out twice", rel)
		}
		seen[rel] = true
	}
}

func TestSaveAttachmentRefusesWhatIsNotAnImage(t *testing.T) {
	v := newVault(t)
	for _, ext := range []string{".exe", ".sh", ".md", "", ".png.exe"} {
		if got, err := v.SaveAttachment(ext, []byte("x")); err == nil {
			t.Errorf("SaveAttachment(%q) = %q, want a refusal", ext, got)
		}
	}
}

func TestSaveAttachmentRefusesNothingAndTooMuch(t *testing.T) {
	v := newVault(t)
	if _, err := v.SaveAttachment(".png", nil); err == nil {
		t.Error("an empty paste was stored")
	}
	if _, err := v.SaveAttachment(".png", make([]byte, MaxAttachment+1)); err == nil {
		t.Error("an oversized image was stored")
	}
}

func TestReadAttachmentRefusesAnythingOutsideTheFolder(t *testing.T) {
	v := newVault(t)
	if err := os.WriteFile(filepath.Join(v.Root(), "secret.md"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"../secret.md", "attachments/../secret.md", "/etc/passwd", "secret.md",
		"attachments/sub/x.png", "attachments/", "", "attachments/x.md",
	} {
		if got, err := v.ReadAttachment(rel); err == nil {
			t.Errorf("ReadAttachment(%q) returned %d bytes, want a refusal", rel, len(got))
		}
	}
}

func TestTheAttachmentsFolderIsNotAPageInTheTree(t *testing.T) {
	v := newVault(t)
	if _, err := v.SaveAttachment(".png", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteRaw("real.md", "# hi"); err != nil {
		t.Fatal(err)
	}
	tree, err := v.Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range tree.Children {
		if c.Name == AttachmentDir {
			t.Error("the attachments folder shows in the sidebar, where it would look like an empty page holder")
		}
	}
	if len(tree.Children) != 1 {
		t.Errorf("tree has %d children, want just the note", len(tree.Children))
	}
}
