package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vedabase-tui/internal/api"
)

var errBadName = errors.New("store: invalid cache name")

// safeName rejects names that could escape the cache directory.
func safeName(name string) bool {
	return name != "" && !strings.Contains(name, "..") &&
		!strings.HasPrefix(name, "/") && !strings.Contains(name, `\`) &&
		!strings.ContainsRune(name, 0)
}

// ItemCache keeps the raw API response of each item at <Dir>/<lang>/<ref>.json,
// so fields the client does not know yet are never lost. OldDir is the
// read-only English cache of earlier versions, used only as an offline fallback.
type ItemCache struct{ Dir, OldDir string }

func (c ItemCache) path(lang, ref string) (string, bool) {
	if !safeName(lang) || !safeName(ref) || strings.Contains(lang, "/") {
		return "", false
	}
	return filepath.Join(c.Dir, lang, filepath.FromSlash(ref)+".json"), true
}

func (c ItemCache) GetRaw(lang, ref string) ([]byte, bool) {
	p, ok := c.path(lang, ref)
	if !ok {
		return nil, false
	}
	b, err := os.ReadFile(p)
	return b, err == nil
}

func (c ItemCache) Has(lang, ref string) bool {
	p, ok := c.path(lang, ref)
	if !ok {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func (c ItemCache) PutRaw(lang, ref string, raw []byte) error {
	p, ok := c.path(lang, ref)
	if !ok {
		return errBadName
	}
	return WriteFileAtomic(p, raw)
}

// GetOld reads the item as an earlier version stored it (English only).
func (c ItemCache) GetOld(ref string) (api.Item, bool) {
	if c.OldDir == "" || !safeName(ref) {
		return api.Item{}, false
	}
	b, err := os.ReadFile(filepath.Join(c.OldDir, filepath.FromSlash(ref)+".json"))
	if err != nil {
		return api.Item{}, false
	}
	var it api.Item
	if json.Unmarshal(b, &it) != nil {
		return api.Item{}, false
	}
	return it, true
}

// BlobCache stores raw bytes under a name that may contain "/"; the file's
// modification time is the saved time.
type BlobCache struct{ Dir string }

func (c BlobCache) path(name string) (string, bool) {
	if !safeName(name) {
		return "", false
	}
	return filepath.Join(c.Dir, filepath.FromSlash(name)), true
}

func (c BlobCache) Get(name string) (data []byte, saved time.Time, ok bool) {
	p, good := c.path(name)
	if !good {
		return nil, time.Time{}, false
	}
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() {
		return nil, time.Time{}, false
	}
	data, err = os.ReadFile(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	return data, st.ModTime(), true
}

func (c BlobCache) Put(name string, data []byte) error {
	p, ok := c.path(name)
	if !ok {
		return errBadName
	}
	return WriteFileAtomic(p, data)
}
