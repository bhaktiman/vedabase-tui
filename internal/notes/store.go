// Package notes is the store for the user's notes on verses. It is user data:
// every write is atomic and a file that cannot be parsed is moved aside, never
// overwritten. It follows internal/collections/store.go.
package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	"vedabase-tui/internal/store"
)

const fileVersion = 1

// Note is the user's text on one reference.
type Note struct {
	Text    string    `json:"text"`
	Updated time.Time `json:"updated"`
}

type fileFormat struct {
	Version int             `json:"version"`
	Notes   map[string]Note `json:"notes"`
}

// Store is safe for concurrent use; Bubble Tea commands may run on any goroutine.
type Store struct {
	mu    sync.Mutex
	path  string
	notes map[string]Note

	// Fingerprint of the file as last loaded or written; nil when it did not
	// exist then. It changes only together with notes (or when the file is
	// known to be gone), so a stale notes never looks current.
	fi os.FileInfo
}

// Open loads path. A missing file gives an empty store and writes nothing. A
// file that cannot be parsed (or has another version) is renamed to
// "<path>.bad-<timestamp>", the warning names it, and the store starts empty.
func Open(path string) (s *Store, warning string, err error) {
	s = &Store{path: path}
	notes, err := s.load()
	if err != nil {
		var pe *parseError
		if !errors.As(err, &pe) {
			return nil, "", err
		}
		bad, mvErr := moveAside(path)
		if mvErr != nil {
			return nil, "", fmt.Errorf("notes: %v; and could not move it aside: %w", err, mvErr)
		}
		s.noteFile() // file is gone now
		s.notes = map[string]Note{}
		return s, fmt.Sprintf("notes file could not be read (%v); kept as %s, starting empty", pe.err, bad), nil
	}
	s.notes = notes
	return s, "", nil
}

func (s *Store) Get(ref string) (Note, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[ref]
	return n, ok
}

func (s *Store) Has(ref string) bool {
	_, ok := s.Get(ref)
	return ok
}

// All returns a copy of every note, keyed by reference; never nil.
func (s *Store) All() map[string]Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]Note, len(s.notes))
	maps.Copy(out, s.notes)
	return out
}

// Set stores text as the note on ref with Updated=now. Text that is empty or
// only whitespace deletes the note (nothing is written if there was none).
// Otherwise the text is stored exactly as given. Text equal to the stored one
// is a no-op: nothing is written and Updated stays.
func (s *Store) Set(ref, text string, now time.Time) error {
	del := strings.TrimSpace(text) == ""
	return s.mutate(func(notes map[string]Note) (map[string]Note, error) {
		have, ok := notes[ref]
		if del {
			if !ok {
				return nil, nil
			}
			delete(notes, ref)
			return notes, nil
		}
		if ok && have.Text == text {
			return nil, nil
		}
		notes[ref] = Note{Text: text, Updated: now.UTC()}
		return notes, nil
	})
}

// mutate reloads the file if another process changed it, runs fn on a copy of
// the notes, writes the result atomically and only then adopts it in memory.
// fn returns nil notes (and nil error) to mean "no change". On any error
// memory is untouched.
func (s *Store) mutate(fn func(notes map[string]Note) (map[string]Note, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed, err := s.fileChanged()
	if err != nil {
		return err
	}
	base := s.notes
	if changed {
		notes, err := s.load()
		var pe *parseError
		switch {
		case err == nil:
			// Memory catches up to what is on disk together with the
			// fingerprint, even if fn or the write below then fails.
			s.notes = notes
			base = notes
		case errors.As(err, &pe):
			// Never overwrite what we cannot read. Memory stays as it was, so
			// the next mutation (file now absent) writes it back.
			bad, mvErr := moveAside(s.path)
			if mvErr != nil {
				return fmt.Errorf("notes: file changed on disk and is unreadable (%v); could not move it aside: %w", pe.err, mvErr)
			}
			// Put our copy back at once so quitting now does not leave only
			// the garbage file. The mutation itself is still refused.
			if wErr := s.write(base); wErr != nil {
				s.noteFile()
				return fmt.Errorf("notes: file changed on disk and is unreadable (%v); kept as %s; could not restore: %w", pe.err, bad, wErr)
			}
			return fmt.Errorf("notes: file changed on disk and is unreadable (%v); kept as %s", pe.err, bad)
		default:
			return err
		}
	}

	next, err := fn(maps.Clone(base))
	if err != nil {
		return err
	}
	if next == nil {
		// No change requested, but adopt what we just reloaded.
		s.notes = base
		return nil
	}
	if err := s.write(next); err != nil {
		return err
	}
	s.notes = next
	return nil
}

// write persists notes atomically and records the new fingerprint. It does not
// touch s.notes.
func (s *Store) write(notes map[string]Note) error {
	data, err := json.MarshalIndent(fileFormat{Version: fileVersion, Notes: notes}, "", "  ")
	if err != nil {
		return err
	}
	if err := store.WriteFileAtomic(s.path, data); err != nil {
		return err
	}
	s.noteFile()
	return nil
}

// fileChanged compares the file on disk with the fingerprint from the last
// load or write. Only a missing file counts as vanished, and that is not a
// change: the in-memory copy is then the only copy, so it is kept and written
// back by the next mutation rather than silently dropped. Any other Stat
// error is returned.
func (s *Store) fileChanged() (bool, error) {
	fi, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s.fi == nil || !os.SameFile(fi, s.fi) || fi.Size() != s.fi.Size() || !fi.ModTime().Equal(s.fi.ModTime()), nil
}

func (s *Store) noteFile() {
	fi, err := os.Stat(s.path)
	if err != nil {
		s.fi = nil
		return
	}
	s.fi = fi
}

type parseError struct{ err error }

func (e *parseError) Error() string { return "notes: " + e.err.Error() }
func (e *parseError) Unwrap() error { return e.err }

// load reads and parses the file, recording its fingerprint. A missing file
// is an empty map; unparsable content or another version is a *parseError.
// The result is never nil.
func (s *Store) load() (map[string]Note, error) {
	fi, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.noteFile()
		return map[string]Note{}, nil
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
	if f.Notes == nil {
		f.Notes = map[string]Note{}
	}
	// Fingerprint from the stat taken before the read: if the file changed in
	// between, the next mutation sees a difference and reloads.
	s.fi = fi
	return f.Notes, nil
}

// moveAside renames path to a fresh "<path>.bad-<timestamp>" name. It links
// then removes instead of renaming so an existing .bad- file is never replaced.
func moveAside(path string) (string, error) {
	base := path + ".bad-" + time.Now().Format("20060102-150405")
	bad := base
	for i := 1; i < 1000; i++ {
		err := os.Link(path, bad)
		if err == nil {
			return bad, os.Remove(path)
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		bad = fmt.Sprintf("%s-%d", base, i)
	}
	return "", errors.New("no free .bad- name")
}
