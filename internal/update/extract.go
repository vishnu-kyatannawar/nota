package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// binaryName is the entry the archives carry: the release workflow packs the
// binary flat, with no directory prefix.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "nota.exe"
	}
	return "nota"
}

// maxBinary caps what will be written out. The real binary is about 17 MB; a
// far larger one means something is wrong, and a decompression bomb should not
// be allowed to fill the disk.
const maxBinary = 200 << 20

// Binary extracts the Nota binary from a downloaded archive into dir and
// returns its path, executable.
//
// Only the exact flat entry name is accepted. Every archive we publish is flat,
// so an entry carrying a path separator means a tampered archive, and writing
// it would let the archive choose where it lands.
func Binary(archive, dir string) (string, error) {
	dest := filepath.Join(dir, binaryName())
	var err error
	if strings.HasSuffix(archive, ".zip") {
		err = fromZip(archive, dest)
	} else {
		err = fromTarGz(archive, dest)
	}
	if err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	return dest, nil
}

// write copies one archive entry out, refusing anything oversized.
func write(dest string, src io.Reader) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dest, err)
	}
	defer func() { _ = f.Close() }()

	n, err := io.Copy(f, io.LimitReader(src, maxBinary+1))
	if err != nil {
		return fmt.Errorf("extracting %s: %w", dest, err)
	}
	if n > maxBinary {
		return fmt.Errorf("%s is larger than %d bytes: refusing to install", binaryName(), maxBinary)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}

func fromTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archive, err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading %s: %w", archive, err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", archive, err)
		}
		if h.Name != binaryName() || h.Typeflag != tar.TypeReg {
			continue
		}
		return write(dest, tr)
	}
	return fmt.Errorf("%s holds no %s", archive, binaryName())
}

func fromZip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archive, err)
	}
	defer func() { _ = r.Close() }()

	for _, e := range r.File {
		if e.Name != binaryName() || e.FileInfo().IsDir() {
			continue
		}
		rc, err := e.Open()
		if err != nil {
			return fmt.Errorf("reading %s from %s: %w", e.Name, archive, err)
		}
		defer func() { _ = rc.Close() }()
		return write(dest, rc)
	}
	return fmt.Errorf("%s holds no %s", archive, binaryName())
}
