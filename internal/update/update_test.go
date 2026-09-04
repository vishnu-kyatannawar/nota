package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// release builds a server that serves a tag, an asset and its checksums, the
// same shapes GitHub and the release workflow produce. Nothing in this file
// touches the real network.
type release struct {
	tag      string
	asset    []byte
	assetSum string // overridden to test a mismatch
	agents   []string
}

func serve(t *testing.T, r *release) (*Client, *release) {
	t.Helper()
	asset, err := AssetName(Version{4, 2, 0})
	if err != nil {
		t.Skipf("no release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	sum := r.assetSum
	if sum == "" {
		h := sha256.Sum256(r.asset)
		sum = hex.EncodeToString(h[:])
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.agents = append(r.agents, req.Header.Get("User-Agent"))
		switch {
		case strings.HasSuffix(req.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name":%q,"name":"Nota"}`, r.tag)
		case strings.HasSuffix(req.URL.Path, "/checksums.txt"):
			// A second platform's line must not be mistaken for ours.
			fmt.Fprintf(w, "%s  %s\n%s  nota_4.2.0_other_amd64.tar.gz\n", sum, asset, strings.Repeat("b", 64))
		case strings.HasSuffix(req.URL.Path, asset):
			_, _ = w.Write(r.asset)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)

	return &Client{HTTP: srv.Client(), Repo: "o/n", API: srv.URL, Download: srv.URL, Agent: "nota/test"}, r
}

func TestLatestReadsTheTag(t *testing.T) {
	c, r := serve(t, &release{tag: "v4.2.0", asset: []byte("x")})
	got, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "v4.2.0" || got.Version != (Version{4, 2, 0}) {
		t.Errorf("Latest() = %+v, want tag v4.2.0", got)
	}
	if !strings.HasSuffix(got.URL, "/releases/tag/v4.2.0") {
		t.Errorf("URL = %q, want the tag page", got.URL)
	}
	if len(r.agents) == 0 || r.agents[0] != "nota/test" {
		t.Errorf("User-Agent = %v, want nota/test — GitHub rejects requests without one", r.agents)
	}
}

func TestLatestRejectsWhatItCannotUse(t *testing.T) {
	tests := []struct {
		name string
		h    http.HandlerFunc
	}{
		{"rate limited", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }},
		{"not json", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "<html>") }},
		{"no tag", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) }},
		{"tag is not a version", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"tag_name":"nightly"}`) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.h)
			defer srv.Close()
			c := &Client{HTTP: srv.Client(), Repo: "o/n", API: srv.URL, Download: srv.URL, Agent: "nota/test"}
			if got, err := c.Latest(context.Background()); err == nil {
				t.Errorf("Latest() = %+v, want an error", got)
			}
		})
	}
}

func TestFetchVerifiesTheChecksum(t *testing.T) {
	body := []byte("pretend this is an archive")
	c, _ := serve(t, &release{tag: "v4.2.0", asset: body})

	var last, total int64
	archive, dir, err := c.Fetch(context.Background(), Release{Tag: "v4.2.0", Version: Version{4, 2, 0}},
		func(done, t int64) { last, total = done, t })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	got, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("downloaded %q, want %q", got, body)
	}
	if last != int64(len(body)) {
		t.Errorf("progress ended at %d, want %d", last, len(body))
	}
	if total != int64(len(body)) {
		t.Errorf("progress total = %d, want %d", total, len(body))
	}
}

func TestFetchRefusesATamperedArchive(t *testing.T) {
	// The published checksum is for something else: the bytes were changed in
	// flight, which is exactly what the check exists to catch.
	c, _ := serve(t, &release{tag: "v4.2.0", asset: []byte("tampered"), assetSum: strings.Repeat("a", 64)})

	archive, dir, err := c.Fetch(context.Background(), Release{Tag: "v4.2.0", Version: Version{4, 2, 0}}, nil)
	if err == nil {
		_ = os.RemoveAll(dir)
		t.Fatalf("Fetch() = %q, want a checksum error", archive)
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %v, want a checksum mismatch", err)
	}
	if archive != "" || dir != "" {
		t.Errorf("Fetch() returned paths %q %q on failure, want nothing left behind", archive, dir)
	}
}

func TestChecksumForMatchesTheExactName(t *testing.T) {
	sum := strings.Repeat("a", 64)
	sums := fmt.Sprintf("%s  nota_4.2.0_linux_amd64.tar.gz\n%s  nota_4.2.0_windows_amd64.zip\n", sum, strings.Repeat("b", 64))

	got, err := checksumFor(sums, "nota_4.2.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != sum {
		t.Errorf("checksumFor() = %q, want %q", got, sum)
	}
	if _, err := checksumFor(sums, "nota_4.2.0_darwin_arm64.tar.gz"); err == nil {
		t.Error("checksumFor() found a hash for an asset that is not published")
	}
	// A truncated hash must not be accepted as a hash.
	if _, err := checksumFor("abc  nota_4.2.0_linux_amd64.tar.gz", "nota_4.2.0_linux_amd64.tar.gz"); err == nil {
		t.Error("checksumFor() accepted a malformed hash")
	}
}

// tarGz builds an archive holding the given entries, flat like the real one.
func tarGz(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "a.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func zipOf(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "a.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBinaryExtractsTheBinaryAndIgnoresTheRest(t *testing.T) {
	dir := t.TempDir()
	entries := map[string]string{binaryName(): "ELF-ish", "README.md": "docs", "LICENSE": "mit"}

	for _, archive := range []string{tarGz(t, dir, entries), zipOf(t, dir, entries)} {
		out := t.TempDir()
		got, err := Binary(archive, out)
		if err != nil {
			t.Fatalf("Binary(%s) returned %v", filepath.Ext(archive), err)
		}
		body, err := os.ReadFile(got)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "ELF-ish" {
			t.Errorf("extracted %q, want the binary", body)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(got)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm()&0o111 == 0 {
				t.Errorf("mode = %v, want it executable", info.Mode())
			}
		}
	}
}

func TestBinaryRefusesAnArchiveThatChoosesWhereItLands(t *testing.T) {
	dir := t.TempDir()
	// A flat archive is all the release workflow ever produces, so a path here
	// means the archive is not ours.
	escape := "../../" + binaryName()
	for _, archive := range []string{
		tarGz(t, dir, map[string]string{escape: "evil"}),
		zipOf(t, dir, map[string]string{escape: "evil"}),
	} {
		if got, err := Binary(archive, t.TempDir()); err == nil {
			t.Errorf("Binary(%s) = %q, want a refusal", filepath.Ext(archive), got)
		}
	}
}

func TestBinaryFailsWhenTheArchiveHasNoBinary(t *testing.T) {
	dir := t.TempDir()
	archive := tarGz(t, dir, map[string]string{"README.md": "docs"})
	if got, err := Binary(archive, t.TempDir()); err == nil {
		t.Errorf("Binary() = %q, want an error", got)
	}
}

func TestApplyReplacesTheTargetAndLeavesTheRunningBinaryAlone(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nota")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hold the outgoing file open the way a running process holds its own
	// image: the swap must not disturb what this handle can read.
	//
	// Only on Unix. Windows lets you rename a running executable but not a file
	// held by an ordinary handle, so a handle is not a stand-in for a running
	// image there — that path is exercised by the real thing, not by this test.
	var running *os.File
	if runtime.GOOS != "windows" {
		var err error
		running, err = os.Open(target)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = running.Close() }()
	}

	staged := filepath.Join(t.TempDir(), "staged")
	if err := os.WriteFile(staged, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(staged, target); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("target = %q, want the new binary", got)
	}
	if running != nil {
		still := make([]byte, 3)
		if _, err := running.ReadAt(still, 0); err != nil {
			t.Fatal(err)
		}
		if string(still) != "old" {
			t.Errorf("the running binary now reads %q, want it untouched at %q", still, "old")
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("installed mode = %v, want it executable", info.Mode())
		}
	}
	// Nothing is left lying around next to the binary.
	if _, err := os.Stat(filepath.Join(dir, ".nota.new")); !os.IsNotExist(err) {
		t.Error("the staging file was left behind")
	}
}

func TestApplyLeavesTheOriginalWhenItCannotStage(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nota")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Apply(filepath.Join(dir, "does-not-exist"), target); err == nil {
		t.Fatal("Apply() succeeded with no staged binary")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Errorf("target = %q, want the original still in place", got)
	}
}

func TestCleanupOldRemovesTheOutgoingBinary(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nota")
	old := target + oldSuffix
	if err := os.WriteFile(old, []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	CleanupOld(target)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the outgoing binary is still there")
	}
	CleanupOld(target) // absent is not an error
}

func TestCanReplaceFollowsWhereTheBinaryActuallyIs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nota")
	if err := os.WriteFile(target, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !CanReplace(target) {
		t.Error("CanReplace() = false for a writable directory")
	}
	if CanReplace(filepath.Join(dir, "no-such-dir", "nota")) {
		t.Error("CanReplace() = true for a directory that does not exist")
	}
}

func TestCanReplaceIsFalseForAReadOnlyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not work this way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	if CanReplace(filepath.Join(locked, "nota")) {
		t.Error("CanReplace() = true for a read-only directory — a package-managed install would be clobbered")
	}
}

func TestAssetNameMatchesWhatTheReleaseWorkflowPublishes(t *testing.T) {
	got, err := AssetName(Version{4, 2, 0})
	if runtime.GOARCH != "amd64" {
		if err == nil {
			t.Errorf("AssetName() = %q on %s, want an error", got, runtime.GOARCH)
		}
		return
	}
	want := map[string]string{
		"linux":   "nota_4.2.0_linux_amd64.tar.gz",
		"windows": "nota_4.2.0_windows_amd64.zip",
	}[runtime.GOOS]
	if want == "" {
		if err == nil {
			t.Errorf("AssetName() = %q on %s, want an error", got, runtime.GOOS)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("AssetName() = %q, want %q", got, want)
	}
}
