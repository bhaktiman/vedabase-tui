package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

const maxBinary = 200 << 20

// readLimited reads r, failing when it yields more than maxBinary bytes.
func readLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBinary+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBinary {
		return nil, fmt.Errorf("update: binary larger than %d bytes", maxBinary)
	}
	return b, nil
}

// extractTarGz returns the regular file named name at the root of a gzip tar.
func extractTarGz(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("update: %s not found in archive", name)
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(h.Name, "./") == name && h.Typeflag == tar.TypeReg {
			return readLimited(tr)
		}
	}
}

// extractZip returns the regular file named name at the root of a zip.
func extractZip(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name != name || !f.Mode().IsRegular() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return readLimited(rc)
	}
	return nil, fmt.Errorf("update: %s not found in archive", name)
}
