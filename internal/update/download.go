package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Supported reports whether a release is published for this platform at all.
// The release workflow builds linux and windows on amd64 and nothing else.
func Supported() bool {
	_, err := AssetName(Version{})
	return err == nil
}

// AssetName is the release asset for the platform this binary was built for.
// The release workflow publishes only linux and windows on amd64; anywhere else
// there is nothing to install and the caller falls back to the release page.
func AssetName(v Version) (string, error) {
	if runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("no release is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("nota_%s_linux_amd64.tar.gz", v), nil
	case "windows":
		return fmt.Sprintf("nota_%s_windows_amd64.zip", v), nil
	default:
		return "", fmt.Errorf("no release is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// checksumFor pulls one asset's expected hash out of a sha256sum-format file:
// "<64 hex>  <bare filename>". The name is matched exactly, so a line for a
// different platform's asset can never be mistaken for ours.
func checksumFor(checksums, asset string) (string, error) {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			sum := strings.ToLower(fields[0])
			if len(sum) != sha256.Size*2 {
				return "", fmt.Errorf("malformed checksum for %s", asset)
			}
			if _, err := hex.DecodeString(sum); err != nil {
				return "", fmt.Errorf("malformed checksum for %s", asset)
			}
			return sum, nil
		}
	}
	return "", fmt.Errorf("no checksum published for %s", asset)
}

// Fetch downloads the release asset into a new temporary directory and checks
// it against the published checksum, returning the archive's path and the
// directory to remove when done.
//
// The checksum proves the download arrived intact. It is not a signature: it
// comes from the same release as the asset, so it cannot vouch for the release
// itself — that trust rests on HTTPS to github.com, exactly as install.sh does.
func (c *Client) Fetch(ctx context.Context, rel Release, progress func(done, total int64)) (archive, dir string, err error) {
	asset, err := AssetName(rel.Version)
	if err != nil {
		return "", "", err
	}
	base := fmt.Sprintf("%s/%s/releases/download/%s", c.Download, c.Repo, rel.Tag)

	dir, err = os.MkdirTemp("", "nota-update-")
	if err != nil {
		return "", "", fmt.Errorf("making a temporary directory: %w", err)
	}
	// Anything short of a verified archive leaves nothing behind.
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
			archive, dir = "", ""
		}
	}()

	sums, err := c.fetchText(ctx, base+"/checksums.txt")
	if err != nil {
		return "", "", err
	}
	want, err := checksumFor(sums, asset)
	if err != nil {
		return "", "", err
	}

	archive = filepath.Join(dir, asset)
	got, err := c.fetchFile(ctx, base+"/"+asset, archive, progress)
	if err != nil {
		return "", "", err
	}
	if got != want {
		return "", "", fmt.Errorf("checksum mismatch for %s: refusing to install", asset)
	}
	return archive, dir, nil
}

func (c *Client) fetchText(ctx context.Context, url string) (string, error) {
	resp, err := c.get(ctx, url, "")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", url, err)
	}
	return string(body), nil
}

// fetchFile streams the body to disk and returns its SHA-256, hashing as it
// goes so the archive is never read twice.
func (c *Client) fetchFile(ctx context.Context, url, dest string, progress func(done, total int64)) (string, error) {
	resp, err := c.get(ctx, url, "")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", dest, err)
	}
	defer func() { _ = f.Close() }()

	sum := sha256.New()
	src := io.Reader(resp.Body)
	if progress != nil {
		src = &counter{r: resp.Body, total: resp.ContentLength, report: progress}
	}
	if _, err := io.Copy(io.MultiWriter(f, sum), src); err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("writing %s: %w", dest, err)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// counter reports download progress without buffering the whole body.
type counter struct {
	r      io.Reader
	done   int64
	total  int64
	report func(done, total int64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	if n > 0 {
		c.report(c.done, c.total)
	}
	return n, err //nolint:wrapcheck // passing the reader's own error through unchanged
}
