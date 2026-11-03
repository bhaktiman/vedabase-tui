package store

import (
	"encoding/json"
	"fmt"
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
	DataDir    string `toml:"data_dir"`
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
# them between computers. Existing files are not moved for you.
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
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	}
	return p
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
		return Settings{ExportDir: filepath.Join(home, "vedabase-exports"), Animations: true, Mouse: true, ReadingWidth: 80}, err
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
// def. A relative setting would depend on the folder the app was started
// from, so it is refused with a warning and def is used.
func DataDir(s Settings, def string) (dir, warning string) {
	switch {
	case s.DataDir == "":
		return def, ""
	case !filepath.IsAbs(s.DataDir):
		home, _ := os.UserHomeDir()
		return def, fmt.Sprintf("config.toml: data_dir %q is not an absolute path; using %s", s.DataDir, tilde(def, home))
	}
	return filepath.Clean(s.DataDir), ""
}

// LeftBehind says which DataFiles are still in old but missing from dir,
// after data_dir moved the data folder, so the user can move them; "" when
// nothing was left behind. It leads with what to do, as a toast may be cut.
func LeftBehind(dir, old string) string {
	if filepath.Clean(dir) == filepath.Clean(old) {
		return ""
	}
	var names []string
	for _, n := range DataFiles {
		if exists(filepath.Join(old, n)) && !exists(filepath.Join(dir, n)) {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ""
	}
	home, _ := os.UserHomeDir()
	return fmt.Sprintf("Move %s from %s to %s (data_dir)",
		strings.Join(names, " and "), tilde(old, home), tilde(dir, home))
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
