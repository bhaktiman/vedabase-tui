package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"vedabase-tui/internal/api"
)

// Settings are the user-editable options in config.toml.
type Settings struct {
	ExportDir  string `toml:"export_dir"` // "~/" expanded on load
	Animations bool   `toml:"animations"`
	Mouse      bool   `toml:"mouse"` // click and wheel; false leaves text selection to the terminal
	// ReadingWidth is the widest line of verse text; 0 means the whole pane.
	ReadingWidth int `toml:"reading_width"`
}

const defaultConfig = `# vedabase-tui settings

# Where Markdown exports are written. A leading "~/" means your home directory.
# export_dir = "~/vedabase-exports"

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
	return s, nil
}

// State is the last reading position and display toggles.
type State struct {
	LastRef    string    `json:"last_ref"`
	Scroll     int       `json:"scroll"`
	Parts      api.Parts `json:"parts"`
	TreeHidden bool      `json:"tree_hidden"`
	Theme      string    `json:"theme"`
	Wide       bool      `json:"wide"` // text uses the whole pane instead of a centred column
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
