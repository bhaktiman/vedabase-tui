// Package collections is the store for the user's named collections, the
// project's only user data. Every write is atomic and a file that cannot be
// parsed is moved aside, never overwritten.
package collections

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"vedabase-tui/internal/store"
)

const fileVersion = 1

type Item struct {
	Ref   string    `json:"ref"`
	Book  string    `json:"book"`
	Label string    `json:"label"`
	Added time.Time `json:"added"`
}

type Collection struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Items   []Item    `json:"items"`
}

type fileFormat struct {
	Version     int          `json:"version"`
	Collections []Collection `json:"collections"`
}

// Store is safe for concurrent use; Bubble Tea commands may run on any goroutine.
type Store struct {
	mu   sync.Mutex
	path string
	cols []Collection

	// Fingerprint of the file as last loaded or written. present is false
	// when the file did not exist at that point.
	present bool
	mtime   time.Time
	size    int64
}

// Open loads path. A missing file gives an empty store and writes nothing. A
// file that cannot be parsed is renamed to "<path>.bad-<timestamp>", the
// warning names it, and the store starts empty.
func Open(path string) (s *Store, warning string, err error) {
	s = &Store{path: path}
	cols, err := s.load()
	if err != nil {
		var pe *parseError
		if !errors.As(err, &pe) {
			return nil, "", err
		}
		bad, mvErr := moveAside(path)
		if mvErr != nil {
			return nil, "", fmt.Errorf("collections: %v; and could not move it aside: %w", err, mvErr)
		}
		s.noteFile() // file is gone now
		return s, fmt.Sprintf("collections file could not be read (%v); kept as %s, starting empty", pe.err, bad), nil
	}
	s.cols = cols
	return s, "", nil
}

func (s *Store) List() []Collection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneAll(s.cols)
}

func (s *Store) Get(name string) (Collection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := find(s.cols, name)
	if i < 0 {
		return Collection{}, false
	}
	return clone(s.cols[i]), true
}

// Add appends it to the named collection, creating the collection if needed.
// An item whose exact Ref is already there is a no-op: added is false.
func (s *Store) Add(name string, it Item) (added bool, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, errors.New("collections: name is blank")
	}
	err = s.mutate(func(cols []Collection) ([]Collection, error) {
		i := find(cols, name)
		if i < 0 {
			cols = append(cols, Collection{Name: name, Created: time.Now().UTC()})
			i = len(cols) - 1
		}
		for _, have := range cols[i].Items {
			if have.Ref == it.Ref {
				return nil, nil // nothing to write
			}
		}
		cols[i].Items = append(cols[i].Items, it)
		added = true
		return cols, nil
	})
	return added && err == nil, err
}

func (s *Store) Remove(name, ref string) error {
	return s.mutate(func(cols []Collection) ([]Collection, error) {
		i := find(cols, name)
		if i < 0 {
			return nil, fmt.Errorf("collections: no collection %q", strings.TrimSpace(name))
		}
		for j, it := range cols[i].Items {
			if it.Ref == ref {
				cols[i].Items = append(cols[i].Items[:j], cols[i].Items[j+1:]...)
				return cols, nil
			}
		}
		return nil, fmt.Errorf("collections: %q is not in %q", ref, cols[i].Name)
	})
}

// Rename fails if newName is blank or is another collection's name. Changing
// only the case of a collection's own name is allowed.
func (s *Store) Rename(old, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("collections: new name is blank")
	}
	return s.mutate(func(cols []Collection) ([]Collection, error) {
		i := find(cols, old)
		if i < 0 {
			return nil, fmt.Errorf("collections: no collection %q", strings.TrimSpace(old))
		}
		if j := find(cols, newName); j >= 0 && j != i {
			return nil, fmt.Errorf("collections: %q already exists", cols[j].Name)
		}
		cols[i].Name = newName
		return cols, nil
	})
}

func (s *Store) Delete(name string) error {
	return s.mutate(func(cols []Collection) ([]Collection, error) {
		i := find(cols, name)
		if i < 0 {
			return nil, fmt.Errorf("collections: no collection %q", strings.TrimSpace(name))
		}
		return append(cols[:i], cols[i+1:]...), nil
	})
}

// Contains reports whether any collection holds exactly ref.
func (s *Store) Contains(ref string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cols {
		for _, it := range c.Items {
			if it.Ref == ref {
				return true
			}
		}
	}
	return false
}

// mutate reloads the file if another process changed it, runs fn on a copy of
// the collections, writes the result atomically and only then adopts it in
// memory. fn returns nil cols (and nil error) to mean "no change". On any
// error memory is untouched.
func (s *Store) mutate(fn func(cols []Collection) ([]Collection, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	base := s.cols
	if s.fileChanged() {
		cols, err := s.load()
		var pe *parseError
		switch {
		case err == nil:
			base = cols
		case errors.As(err, &pe):
			// Never overwrite what we cannot read. Memory stays as it was, so
			// the next mutation (file now absent) writes it back.
			bad, mvErr := moveAside(s.path)
			if mvErr != nil {
				return fmt.Errorf("collections: file changed on disk and is unreadable (%v); could not move it aside: %w", pe.err, mvErr)
			}
			s.noteFile()
			return fmt.Errorf("collections: file changed on disk and is unreadable (%v); kept as %s", pe.err, bad)
		default:
			return err
		}
	}

	next, err := fn(cloneAll(base))
	if err != nil {
		return err
	}
	if next == nil {
		// No change requested, but adopt what we just reloaded.
		s.cols = base
		return nil
	}
	data, err := json.MarshalIndent(fileFormat{Version: fileVersion, Collections: next}, "", "  ")
	if err != nil {
		return err
	}
	if err := store.WriteFileAtomic(s.path, data); err != nil {
		return err
	}
	s.cols = next
	s.noteFile()
	return nil
}

// fileChanged compares the file on disk with the fingerprint from the last
// load or write. A file that vanished is not a change: the in-memory copy is
// then the only copy, so it is kept and written back by the next mutation
// rather than silently dropping the user's data. (A same-size rewrite within
// one mtime tick goes unnoticed; that is the accepted limit of the check.)
func (s *Store) fileChanged() bool {
	fi, err := os.Stat(s.path)
	if err != nil {
		return false
	}
	return !s.present || fi.Size() != s.size || !fi.ModTime().Equal(s.mtime)
}

func (s *Store) noteFile() {
	fi, err := os.Stat(s.path)
	if err != nil {
		s.present, s.mtime, s.size = false, time.Time{}, 0
		return
	}
	s.present, s.mtime, s.size = true, fi.ModTime(), fi.Size()
}

type parseError struct{ err error }

func (e *parseError) Error() string { return "collections: " + e.err.Error() }
func (e *parseError) Unwrap() error { return e.err }

// load reads and parses the file, recording its fingerprint. A missing file
// is an empty list; unparsable content is a *parseError.
func (s *Store) load() ([]Collection, error) {
	fi, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.noteFile()
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, &parseError{err}
	}
	if f.Version != fileVersion {
		return nil, &parseError{fmt.Errorf("unsupported version %d", f.Version)}
	}
	// Fingerprint from the stat taken before the read: if the file changed in
	// between, the next mutation sees a difference and reloads.
	s.present, s.mtime, s.size = true, fi.ModTime(), fi.Size()
	return f.Collections, nil
}

func moveAside(path string) (string, error) {
	base := path + ".bad-" + time.Now().Format("20060102-150405")
	bad := base
	for i := 1; ; i++ {
		if _, err := os.Lstat(bad); errors.Is(err, os.ErrNotExist) {
			break
		}
		bad = fmt.Sprintf("%s-%d", base, i)
	}
	return bad, os.Rename(path, bad)
}

func find(cols []Collection, name string) int {
	name = strings.TrimSpace(name)
	for i, c := range cols {
		if strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}

func clone(c Collection) Collection {
	c.Items = append(make([]Item, 0, len(c.Items)), c.Items...) // never nil: JSON stays []
	return c
}

func cloneAll(cols []Collection) []Collection {
	out := make([]Collection, len(cols))
	for i, c := range cols {
		out[i] = clone(c)
	}
	return out
}
