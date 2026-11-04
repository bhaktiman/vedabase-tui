package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"vedabase-tui/internal/api"
)

// Settings are the user-editable options in config.toml.
type Settings struct {
	ExportDir string `toml:"export_dir"` // "~/" expanded on load
	// DataDir holds collections.json and notes.json; "" means the XDG data
	// folder. "~/" is expanded on load.
	DataDir string `toml:"data_dir"`
	// DataDirErr is set when config.toml has a data_dir line that could not
	// be read: the data folder is then unknown, not the default.
	DataDirErr string `toml:"-"`
	Animations bool   `toml:"animations"`
	Mouse      bool   `toml:"mouse"` // click and wheel; false leaves text selection to the terminal
	// ReadingWidth is the widest line of verse text; 0 means the whole pane.
	ReadingWidth int `toml:"reading_width"`
}

const defaultConfig = `# vedabase-tui settings

# Where Markdown exports are written. A leading "~/" means your home directory.
# export_dir = "~/vedabase-exports"

# Where your collections and notes are kept. A leading "~/" means your home
# directory. Point it at a synced folder (Dropbox, OneDrive, Syncthing) to share
# them between computers. Existing files are not moved for you. On Windows
# use single quotes or forward slashes: 'C:\Users\me\Dropbox\vb'.
# data_dir = "~/.local/share/vedabase-tui"

# Set to false to turn off all motion (fades, highlights, slides).
# animations = true

# Set to false to turn off mouse support, so your terminal's own text
# selection works without holding Shift.
# mouse = true

# Widest line of verse text, in columns; the text is centred. 0 = use the whole pane.
# reading_width = 80
`

func expandHome(p, home string) string {
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"), strings.HasPrefix(p, "~"+string(filepath.Separator)):
		return filepath.Join(home, p[2:])
	}
	return p
}

// salvageDataDir reads the data_dir line on its own when the whole file does
// not parse, so one bad setting elsewhere never moves the data folder.
func salvageDataDir(b []byte, home string) (dir, problem string) {
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(t, "data_dir")
		if !ok || !strings.HasPrefix(strings.TrimSpace(rest), "=") {
			continue
		}
		var x struct {
			DataDir string `toml:"data_dir"`
		}
		if err := toml.Unmarshal([]byte(t), &x); err != nil {
			return "", err.Error()
		}
		return expandHome(x.DataDir, home), ""
	}
	return "", ""
}

// LoadSettings reads path. A missing file yields the defaults and writes a
// commented default file so the options are discoverable.
func LoadSettings(path string) (Settings, error) {
	home, _ := os.UserHomeDir()
	s := Settings{ExportDir: filepath.Join(home, "vedabase-exports"), Animations: true, Mouse: true, ReadingWidth: 80}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if werr := WriteFileAtomic(path, []byte(defaultConfig)); werr != nil {
			return s, werr
		}
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := toml.Unmarshal(b, &s); err != nil {
		d := Settings{ExportDir: filepath.Join(home, "vedabase-exports"), Animations: true, Mouse: true, ReadingWidth: 80}
		d.DataDir, d.DataDirErr = salvageDataDir(b, home)
		return d, err
	}
	if s.ReadingWidth < 0 {
		s.ReadingWidth = 80
	} else if s.ReadingWidth > 0 && s.ReadingWidth < 30 {
		s.ReadingWidth = 30
	}
	s.ExportDir = expandHome(s.ExportDir, home)
	s.DataDir = expandHome(s.DataDir, home)
	return s, nil
}

// DataFiles are the files kept in the data folder.
var DataFiles = []string{"collections.json", "notes.json"}

// DataDir is the folder for DataFiles: the data_dir setting when set, else
// def. ok is false when data_dir is set but unusable (unreadable, not a full
// path, not a folder): collections and notes must then stay off for the run,
// since saving them in def would strand them where the app will not look once
// data_dir is fixed. warning is for a toast.
func DataDir(s Settings, def string) (dir, warning string, ok bool) {
	home, _ := os.UserHomeDir()
	const off = "; collections and notes are off until it is fixed"
	switch {
	case s.DataDirErr != "":
		return "", "config.toml: data_dir could not be read (" + s.DataDirErr + ")" + off, false
	case s.DataDir == "":
		return def, "", true
	case !filepath.IsAbs(s.DataDir):
		return "", fmt.Sprintf("config.toml: data_dir %q is not a full path (start it with ~/ or /)", s.DataDir) + off, false
	}
	dir = filepath.Clean(s.DataDir)
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, fmt.Sprintf("data_dir %s does not exist yet: it is created when you first save", tilde(dir, home)), true
	case err != nil:
		return "", fmt.Sprintf("data_dir %s: %v", tilde(dir, home), err) + off, false
	case !fi.IsDir():
		return "", fmt.Sprintf("data_dir %s is not a folder", tilde(dir, home)) + off, false
	}
	return dir, "", true
}

// LeftBehind names the DataFiles still in old when data_dir points elsewhere:
// the app no longer reads them, so the user should move or merge them; ""
// when there are none.
func LeftBehind(dir, old string) string {
	if sameDir(dir, old) {
		return ""
	}
	var names []string
	for _, n := range DataFiles {
		if exists(filepath.Join(old, n)) {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ""
	}
	home, _ := os.UserHomeDir()
	return fmt.Sprintf("Not used: %s in %s (data_dir is %s)",
		strings.Join(names, " and "), tilde(old, home), tilde(dir, home))
}

func sameDir(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(fa, fb)
}

// tilde writes paths under home as ~/….
func tilde(p, home string) string {
	if home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return "~" + string(filepath.Separator) + rel
		}
	}
	return p
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// State is the last reading position and display toggles.
type State struct {
	LastRef    string    `json:"last_ref"`
	Scroll     int       `json:"scroll"`
	Parts      api.Parts `json:"parts"`
	TreeHidden bool      `json:"tree_hidden"`
	Theme      string    `json:"theme"`
	Lang       string    `json:"lang"` // text language: en, es, pt, hi or ru; empty means en
	Wide       bool      `json:"wide"` // text uses the whole pane instead of a centred column
	HideHints  bool      `json:"hide_hints"`
	Zen        bool      `json:"zen"`                  // distraction-free reading: only the text column
	TreeWidth  int       `json:"tree_width,omitempty"` // sidebar width dragged by the user; 0 = automatic
	// CopyOff lists the parts the `y` menu leaves unticked, comma-separated
	// (devanagari, verse, translation, purport, text, link); empty = all on.
	CopyOff string `json:"copy_off,omitempty"`
	// PartsSet marks Parts as chosen by the user, so that "all off" is not mistaken
	// for an old file with no parts saved.
	PartsSet bool `json:"parts_set"`
}

// LoadState returns the saved state; a missing or corrupt file gives defaults.
func LoadState(path string) State {
	def := State{Parts: api.AllParts()}
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var s State
	if json.Unmarshal(b, &s) != nil {
		return def
	}
	return s
}

func SaveState(path string, s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, b)
}
