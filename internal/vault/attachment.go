package vault

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// AttachmentDir is the vault folder holding pasted images. It sits at the root
// rather than inside .nota so the files stay yours: a note's markdown refers to
// "attachments/x.png" relative to the vault, which is the convention other
// markdown tools already follow.
const AttachmentDir = "attachments"

// MaxAttachment is the largest file that will be stored. Pasted screenshots are
// a few megabytes; anything past this is a mistake worth refusing.
const MaxAttachment = 25 << 20

// Extensions an image paste may carry. Anything else is refused rather than
// written, so the vault cannot be used to stash arbitrary files.
var imageExt = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml",
	".avif": "image/avif", ".bmp": "image/bmp",
}

// IsImage reports whether this extension may be stored as an attachment.
func IsImage(ext string) bool { return imageExt[strings.ToLower(ext)] != "" }

// ContentType is the type to serve a stored attachment as.
func ContentType(name string) string {
	return imageExt[strings.ToLower(filepath.Ext(name))]
}

// SaveAttachment writes an image into the vault and returns the path a note
// should refer to, relative to the vault root.
//
// The name is never taken from the paste: a clipboard image arrives with no
// name at all, and one that did arrive with a name could collide or escape.
func (v *Vault) SaveAttachment(ext string, data []byte) (string, error) {
	ext = strings.ToLower(ext)
	if !IsImage(ext) {
		return "", fmt.Errorf("%s is not an image", ext)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("nothing to save")
	}
	if len(data) > MaxAttachment {
		return "", fmt.Errorf("image is larger than %d MB", MaxAttachment>>20)
	}

	dir := filepath.Join(v.root, AttachmentDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("making %s: %w", AttachmentDir, err)
	}
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("naming the attachment: %w", err)
	}
	name := fmt.Sprintf("%s-%s%s", time.Now().Format("20060102"), hex.EncodeToString(b[:]), ext)

	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("writing the attachment: %w", err)
	}
	return path.Join(AttachmentDir, name), nil
}

// ReadAttachment returns a stored attachment. The path must name a file
// directly inside the attachments folder — no subdirectories, no climbing out.
func (v *Vault) ReadAttachment(rel string) ([]byte, error) {
	clean := path.Clean(strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "/"))
	dir, name := path.Split(clean)
	if strings.Trim(dir, "/") != AttachmentDir || name == "" || name == "." || name == ".." {
		return nil, fmt.Errorf("%w: %s", ErrOutsideVault, rel)
	}
	if !IsImage(filepath.Ext(name)) {
		return nil, fmt.Errorf("%w: %s", ErrOutsideVault, rel)
	}
	data, err := os.ReadFile(filepath.Join(v.root, AttachmentDir, name))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", rel, err)
	}
	return data, nil
}
