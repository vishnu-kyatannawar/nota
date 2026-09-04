package services

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/vishnu-kyatannawar/nota/internal/vault"
)

// AttachmentService stores images pasted into a note.
type AttachmentService struct {
	core *Core
}

// NewAttachmentService returns the service bound as AttachmentService.
func NewAttachmentService(core *Core) *AttachmentService { return &AttachmentService{core: core} }

// Save writes a pasted image into the vault and returns the path to put in the
// markdown. The path is relative to the vault root — "attachments/x.png" — so
// the same link resolves in the app, in another markdown editor, and in the
// file tree itself.
//
// The image arrives base64-encoded because that is what crosses the bindings;
// it is decoded here and written as bytes.
func (a *AttachmentService) Save(ext, data string) (string, error) {
	// A data URL may arrive whole; keep only the payload.
	if i := strings.Index(data, ";base64,"); i >= 0 {
		data = data[i+len(";base64,"):]
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		return "", fmt.Errorf("decoding the pasted image: %w", err)
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return a.core.vault.SaveAttachment(ext, raw)
}

// AttachmentHandler serves stored images to the webview.
//
// A note's markdown says "attachments/x.png", and the page is served from the
// root, so the webview asks for "/attachments/x.png" without anything having to
// rewrite the link. Everything else falls through to the bundled assets.
func AttachmentHandler(core *Core, next http.Handler) http.Handler {
	prefix := "/" + vault.AttachmentDir + "/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, prefix) {
			next.ServeHTTP(w, r)
			return
		}
		data, err := core.vault.ReadAttachment(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if ct := vault.ContentType(r.URL.Path); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		// The name carries random bytes and the file never changes under it.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(data)
	})
}
