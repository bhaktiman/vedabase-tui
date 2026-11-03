package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"vedabase-tui/internal/render"
	"vedabase-tui/internal/store"
)

func ctrlF() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl} }

func TestFindMatchesFoldAndColumns(t *testing.T) {
	var r Reader
	r.SetSize(40, 5)
	r.setHighlight(render.Palette{Gold: lipgloss.Color("#e8a95b"), Bg: lipgloss.Color("#000000")})
	r.SetContent("plain Kṛṣṇa here\n\x1b[1mKRSNA\x1b[0m and kṛṣṇa\nदेवता कृष्ण", nil, false)
	r.SetFind("krsna", 0)
	if len(r.matches) != 3 {
		t.Fatalf("matches = %+v", r.matches)
	}
	if m := r.matches[0]; m.line != 0 || m.c0 != 6 || m.c1 != 11 {
		t.Errorf("first = %+v", m)
	}
	// Devanagari: the match must start after the preceding word's cells.
	r.SetFind("कृष्ण", 0)
	if len(r.matches) != 1 || r.matches[0].c0 != ansi.StringWidth("देवता ") {
		t.Errorf("devanagari = %+v", r.matches)
	}
	// Highlighting keeps the text and styled lines intact.
	r.SetFind("krsna", 0)
	for i, l := range r.Lines()[:3] {
		want := []string{"plain Kṛṣṇa here", "KRSNA and kṛṣṇa", "देवता कृष्ण"}[i]
		if got := strings.TrimRight(ansi.Strip(l), " "); got != want {
			t.Errorf("line %d = %q, want %q", i, got, want)
		}
	}
	if !strings.Contains(r.Lines()[0], "\x1b[") {
		t.Error("match not styled")
	}
}

func openFind(t *testing.T) *App {
	t.Helper()
	a, sv := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.opener = func(string) error { return nil }
	findSaved = sv
	return a
}

var findSaved *saved

func TestFindLiveCounterAndStepping(t *testing.T) {
	a := openFind(t)
	drive(a, ctrlF())
	if !a.findBar.visible {
		t.Fatal("ctrl+f did not open the bar")
	}
	for _, c := range "the" {
		drive(a, key(string(c)))
	}
	if n := len(a.reader.matches); n < 2 {
		t.Fatalf("matches = %d", n)
	}
	if v := plain(a); !strings.Contains(v, "find: the") || !strings.Contains(v, "1/") {
		t.Errorf("bar lacks counter:\n%s", v)
	}
	drive(a, key("enter"))
	if a.findBar.visible || a.reader.find == "" {
		t.Fatal("enter should close the input and keep the find")
	}
	drive(a, key("n"))
	if a.reader.cur != 1 {
		t.Errorf("cur after n = %d", a.reader.cur)
	}
	drive(a, key("N"), key("N"))
	if a.reader.cur != len(a.reader.matches)-1 {
		t.Errorf("N should wrap, cur = %d", a.reader.cur)
	}
	m := a.reader.matches[a.reader.cur]
	if m.line < a.reader.offset || m.line >= a.reader.offset+a.reader.h {
		t.Errorf("current match line %d not visible at offset %d", m.line, a.reader.offset)
	}
	drive(a, key("esc"))
	if a.reader.find != "" || len(a.reader.matches) != 0 {
		t.Error("esc did not clear the find")
	}
}

func TestFindNoMatchesAndEscInInput(t *testing.T) {
	a := openFind(t)
	drive(a, ctrlF())
	for _, c := range "zzzqx" {
		drive(a, key(string(c)))
	}
	if v := plain(a); !strings.Contains(v, "no matches") {
		t.Errorf("view lacks 'no matches':\n%s", v)
	}
	drive(a, key("esc"))
	if a.findBar.visible || a.reader.find != "" {
		t.Error("esc in the input should clear at once")
	}
}

func TestFindSwallowsKeys(t *testing.T) {
	a := openFind(t)
	saves := len(findSaved.calls)
	drive(a, ctrlF())
	for _, c := range "q?/scRDdJKya" {
		drive(a, key(string(c)))
	}
	if len(findSaved.calls) != saves || a.help || a.search.visible || a.picker.visible || a.copyMenu.visible {
		t.Fatal("a typed key fired an action")
	}
	if got := a.findBar.input.Value(); got != "q?/scRDdJKya" {
		t.Errorf("input = %q", got)
	}
}

func TestFindDiacriticsAndLifecycle(t *testing.T) {
	a := openFind(t)
	drive(a, ctrlF())
	for _, c := range "krsna" {
		drive(a, key(string(c)))
	}
	if len(a.reader.matches) == 0 {
		t.Fatal("krsna found nothing")
	}
}

func TestFindSurvivesResizeAndClearsOnOpen(t *testing.T) {
	a := openFind(t)
	drive(a, ctrlF())
	for _, c := range "the" {
		drive(a, key(string(c)))
	}
	drive(a, key("enter"))
	drive(a, tea.WindowSizeMsg{Width: 90, Height: 40})
	if a.reader.find != "the" || len(a.reader.matches) == 0 {
		t.Errorf("resize lost the find: %q %d", a.reader.find, len(a.reader.matches))
	}
	drive(a, key("l"))
	if a.reader.find != "" {
		t.Error("opening another item should clear the find")
	}
}

func TestFindNeedsItem(t *testing.T) {
	a := openFind(t)
	a.hasItem = false
	drive(a, ctrlF())
	if a.findBar.visible {
		t.Error("ctrl+f opened with no item")
	}
}
