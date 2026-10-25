package ui

import (
	"encoding/json"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
)

// key builds the key press a terminal would send for s.
func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+n":
		return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "ctrl+d":
		return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	}
	r := []rune(s)
	if len(r) != 1 {
		panic("key: unknown key " + s)
	}
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

// sampleLibrary is the sample sitemap with the real book titles.
func sampleLibrary(t *testing.T) *library.Library {
	t.Helper()
	data, err := os.ReadFile("../api/testdata/books.json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Books []api.Book `json:"books"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("../library/testdata/sitemap-sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lib, err := library.Parse(f, env.Books)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}
