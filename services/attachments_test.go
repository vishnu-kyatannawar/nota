package services

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func attachmentCore(t *testing.T) (*AttachmentService, *Core) {
	t.Helper()
	core := newTestCore(t)
	return NewAttachmentService(core), core
}

func TestSaveStoresAPastedImage(t *testing.T) {
	a, _ := attachmentCore(t)
	rel, err := a.Save(".png", base64.StdEncoding.EncodeToString([]byte("\x89PNG")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rel, "attachments/") {
		t.Errorf("path = %q, want one under attachments/", rel)
	}
}

func TestSaveAcceptsAWholeDataUrlAndABareExtension(t *testing.T) {
	a, _ := attachmentCore(t)
	body := base64.StdEncoding.EncodeToString([]byte("\x89PNG"))
	if _, err := a.Save("png", "data:image/png;base64,"+body); err != nil {
		t.Fatalf("a data URL with no leading dot on the extension was refused: %v", err)
	}
}

func TestSaveRefusesRubbish(t *testing.T) {
	a, _ := attachmentCore(t)
	if _, err := a.Save(".png", "not base64!!"); err == nil {
		t.Error("undecodable data was stored")
	}
	if _, err := a.Save(".exe", base64.StdEncoding.EncodeToString([]byte("MZ"))); err == nil {
		t.Error("a non-image was stored")
	}
}

func TestTheHandlerServesAnImageAndNothingElse(t *testing.T) {
	a, core := attachmentCore(t)
	rel, err := a.Save(".png", base64.StdEncoding.EncodeToString([]byte("\x89PNG body")))
	if err != nil {
		t.Fatal(err)
	}
	fallthroughHit := false
	h := AttachmentHandler(core, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallthroughHit = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+rel, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "\x89PNG body" {
		t.Errorf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
	if fallthroughHit {
		t.Error("an attachment request reached the bundled assets")
	}

	// Anything outside the folder is a 404, not a file read.
	for _, p := range []string{"/attachments/../secret.md", "/attachments/nope.png", "/attachments/x.md"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code == http.StatusOK {
			t.Errorf("%s was served", p)
		}
	}

	// Everything else still goes to the app.
	fallthroughHit = false
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if !fallthroughHit {
		t.Error("an ordinary request did not reach the bundled assets")
	}
}
