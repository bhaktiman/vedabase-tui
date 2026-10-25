// Package store holds everything vedabase-tui keeps on disk: XDG paths,
// the item and blob caches, settings and UI state.
package store

import (
	"errors"
	"os"
	"path/filepath"
)

const appDir = "vedabase-tui"

// Paths are the per-purpose directories; each already ends in "vedabase-tui".
type Paths struct{ Cache, Data, Config, State string }

// DefaultPaths resolves the XDG base directories, falling back to the
// defaults under $HOME when the variables are unset or not absolute.
func DefaultPaths() (Paths, error) {
	home, _ := os.UserHomeDir()
	dir := func(env, fallback string) (string, error) {
		if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
			return filepath.Join(v, appDir), nil
		}
		if home == "" {
			return "", errors.New("store: cannot determine home directory and " + env + " is unset")
		}
		return filepath.Join(home, fallback, appDir), nil
	}
	var p Paths
	var err error
	if p.Cache, err = dir("XDG_CACHE_HOME", ".cache"); err != nil {
		return Paths{}, err
	}
	if p.Data, err = dir("XDG_DATA_HOME", ".local/share"); err != nil {
		return Paths{}, err
	}
	if p.Config, err = dir("XDG_CONFIG_HOME", ".config"); err != nil {
		return Paths{}, err
	}
	if p.State, err = dir("XDG_STATE_HOME", ".local/state"); err != nil {
		return Paths{}, err
	}
	return p, nil
}

// WriteFileAtomic replaces path with data so that readers see either the old
// or the new content, never a partial file: temp file in the same directory,
// fsync, rename, fsync the directory.
func WriteFileAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Chmod(0o644); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
