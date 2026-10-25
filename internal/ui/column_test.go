package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

func bootCol(t *testing.T, w, h int, st store.State, width int) *App {
	t.Helper()
	if st.Parts == (api.Parts{}) {
		st.Parts = api.AllParts()
	}
	st.LastRef = "bg/2/13"
	a := New(Options{Backend: newFake(t), State: st,
		Settings: store.Settings{ReadingWidth: width}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: w, Height: h}, a.Init()())
	return a
}

// readerRows is the text inside the reader frame, one string per row.
func readerRows(a *App) []string {
	lines := strings.Split(plain(a), "\n")
	var out []string
	for _, l := range lines[1 : len(lines)-1] {
		l = strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(l, "│"), "│"), "┃") // ┃ is the scroll thumb
		out = append(out, l)
	}
	return out[1 : len(out)-1]
}

func maxTextW(rows []string) (w, firstIndent int) {
	firstIndent = -1
	for _, r := range rows {
		if strings.TrimSpace(r) == "" {
			continue
		}
		if firstIndent < 0 {
			firstIndent = len(r) - len(strings.TrimLeft(r, " "))
		}
		w = max(w, lipgloss.Width(strings.TrimSpace(r)))
	}
	return
}

func TestCentredColumn(t *testing.T) {
	a := bootCol(t, 200, 40, store.State{TreeHidden: true}, 80)
	inner := a.readerW()
	w, indent := maxTextW(readerRows(a))
	if w == 0 || w > 80 {
		t.Fatalf("text width %d", w)
	}
	if want := (inner - 80) / 2; indent < want || indent > want+40 {
		t.Fatalf("indent %d, want about %d", indent, want)
	}
	if a.colIndent() != (inner-80)/2 {
		t.Fatalf("colIndent %d", a.colIndent())
	}

	drive(a, key("w"))
	if a.toasts.cur == nil || a.toasts.cur.text != "Full width" {
		t.Fatalf("toast %+v", a.toasts.cur)
	}
	if a.renderedW != inner {
		t.Fatalf("not re-rendered full width: %d", a.renderedW)
	}
	if !a.State().Wide {
		t.Fatal("wide not in saved state")
	}
	if _, in := maxTextW(readerRows(a)); in >= (inner-80)/2 {
		t.Fatalf("still indented: %d", in)
	}
	b := bootCol(t, 200, 40, a.State(), 80)
	if !b.state.Wide || b.textW() != b.readerW() {
		t.Fatal("new App did not start wide")
	}
	drive(a, key("w"))
	if a.State().Wide || a.renderedW != 80 {
		t.Fatal("w did not return to the column")
	}
}

func TestColumnNarrowUnchanged(t *testing.T) {
	narrow := bootCol(t, 90, 30, store.State{}, 80)
	wide := bootCol(t, 90, 30, store.State{Wide: true}, 80)
	if narrow.textW() != narrow.readerW() {
		t.Fatalf("textW %d readerW %d", narrow.textW(), narrow.readerW())
	}
	if plain(narrow) != plain(wide) {
		t.Fatal("narrow pane differs between modes")
	}
}

func TestReadingWidthSetting(t *testing.T) {
	load := func(body string) int {
		p := filepath.Join(t.TempDir(), "config.toml")
		if body != "" {
			os.WriteFile(p, []byte(body), 0o644)
		}
		s, err := store.LoadSettings(p)
		if err != nil {
			t.Fatal(err)
		}
		return s.ReadingWidth
	}
	for body, want := range map[string]int{"": 80, "animations = true\n": 80, "reading_width = 60\n": 60,
		"reading_width = 0\n": 0, "reading_width = 10\n": 30} {
		if got := load(body); got != want {
			t.Errorf("%q: %d, want %d", body, got, want)
		}
	}
	a := bootCol(t, 200, 40, store.State{TreeHidden: true}, 60)
	if a.textW() != 60 {
		t.Fatalf("textW %d", a.textW())
	}
	a = bootCol(t, 200, 40, store.State{TreeHidden: true}, 0)
	if a.textW() != a.readerW() {
		t.Fatal("0 should use the whole pane")
	}
}
