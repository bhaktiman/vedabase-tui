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

// ItemCache stores one JSON file per ref at <Dir>/<ref>.json.
type ItemCache struct{ Dir string }

func (c ItemCache) path(ref string) (string, bool) {
	if !safeName(ref) {
		return "", false
	}
	return filepath.Join(c.Dir, filepath.FromSlash(ref)+".json"), true
}

func (c ItemCache) Get(ref string) (api.Item, bool) {
	p, ok := c.path(ref)
	if !ok {
		return api.Item{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return api.Item{}, false
	}
	var it api.Item
	if json.Unmarshal(b, &it) != nil {
		return api.Item{}, false
	}
	return it, true
}

func (c ItemCache) Has(ref string) bool {
	p, ok := c.path(ref)
	if !ok {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func (c ItemCache) Put(it api.Item) error {
	p, ok := c.path(it.Ref)
	if !ok {
		return errBadName
	}
	b, err := json.Marshal(it)
	if err != nil {
		return err
	}
	return WriteFileAtomic(p, b)
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
