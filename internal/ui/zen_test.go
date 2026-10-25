package ui

import (
	"strings"
	"testing"

	"vedabase-tui/internal/store"
)

func TestHideHints(t *testing.T) {
	a := bootCol(t, 200, 30, store.State{}, 80)
	if !strings.Contains(bottomLine2(a), "/ search") {
		t.Fatal("hints should show by default")
	}
	drive(a, key("H"))
	txt := bottomLine2(a)
	if strings.Contains(txt, "/ search") || strings.Contains(txt, "? help") {
		t.Fatalf("hints still shown: %q", txt)
	}
	for _, p := range []string{"1 dev", "2 verse", "5 purport", "Reading"} {
		if !strings.Contains(txt, p) {
			t.Fatalf("bar lost %q: %q", p, txt)
		}
	}
	if !strings.Contains(plain(a), "Hints hidden") {
		t.Fatal("no toast")
	}
	// A part label is still clickable.
	_, hits := a.bottomLayout()
	before := a.parts.Verse
	drive(a, click(hits[1].x0, a.h-1))
	if a.parts.Verse == before {
		t.Fatal("part toggle not clickable")
	}
	st := a.State()
	if !st.HideHints {
		t.Fatal("not saved")
	}
	b := bootCol(t, 200, 30, st, 80)
	if strings.Contains(bottomLine2(b), "/ search") {
		t.Fatal("new app should start hidden")
	}
	drive(b, key("H"))
	if !strings.Contains(bottomLine2(b), "/ search") || !strings.Contains(plain(b), "Hints shown") {
		t.Fatal("H again should show hints")
	}
}

func bottomLine2(a *App) string { return ansiRe.ReplaceAllString(bottomLine(a), "") }

func TestZenMode(t *testing.T) {
	a := bootCol(t, 160, 45, store.State{}, 80)
	a.focusTree = true
	drive(a, key("F"))
	v := plain(a)
	for _, bad := range []string{"╭", "│", "┃", "●", "vedabase.cc", "Gita", "/ search", "dev"} {
		if strings.Contains(v, bad) {
			t.Fatalf("zen view has %q:\n%s", bad, v)
		}
	}
	if a.focusTree {
		t.Fatal("focus should move to the reader")
	}
	lines := strings.Split(v, "\n")
	if len(lines) != 45 {
		t.Fatalf("%d rows", len(lines))
	}
	if strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[44]) != "" {
		t.Fatal("first and last rows should be blank")
	}
	if strings.TrimSpace(lines[1]) == "" {
		t.Fatalf("text should start on row 1:\n%s", v)
	}
	if a.readerW() != 160 {
		t.Fatalf("readerW %d", a.readerW())
	}
	w, indent := maxTextW(lines[1:44])
	if w > 80 || indent != (160-a.textW())/2 {
		t.Fatalf("column width %d indent %d", w, indent)
	}
	ref := a.ref
	drive(a, key("l"))
	if a.ref == ref {
		t.Fatal("l should change item")
	}
	drive(a, key("h"))
	if a.ref != ref {
		t.Fatal("h should change item back")
	}
	drive(a, key("t"), key("tab"), key("z"))
	if a.state.TreeHidden || a.focusTree {
		t.Fatal("t/tab/z must do nothing in zen")
	}
	drive(a, key("F"))
	v = plain(a)
	for _, want := range []string{"╭", "●", "Contents", "vedabase.cc"} {
		if !strings.Contains(v, want) {
			t.Fatalf("leaving zen lacks %q", want)
		}
	}
	if !a.treeVisible() || !a.focusTree {
		t.Fatal("sidebar visibility and focus should be restored")
	}
	// Hidden sidebar stays hidden.
	b := bootCol(t, 160, 45, store.State{TreeHidden: true}, 80)
	drive(b, key("F"), key("F"))
	if b.treeVisible() {
		t.Fatal("sidebar came back")
	}
}

func TestZenEscLeaves(t *testing.T) {
	a := bootCol(t, 160, 45, store.State{}, 80)
	drive(a, key("F"))
	drive(a, key("esc"))
	if a.state.Zen || !strings.Contains(plain(a), "╭") {
		t.Fatal("esc should leave zen")
	}
}

func TestZenPersists(t *testing.T) {
	a := bootCol(t, 160, 45, store.State{}, 80)
	drive(a, key("F"))
	st := a.State()
	if !st.Zen {
		t.Fatal("not saved")
	}
	b := bootCol(t, 160, 45, st, 80)
	if strings.Contains(plain(b), "╭") || !b.state.Zen {
		t.Fatal("should start in zen")
	}
}

func TestZenOverlaysStillWork(t *testing.T) {
	a := bootCol(t, 160, 45, store.State{}, 80)
	drive(a, key("F"), key("/"))
	if !a.search.visible {
		t.Fatal("/ should open search")
	}
	drive(a, key("esc"))
	if a.search.visible || !a.state.Zen {
		t.Fatal("first esc closes search, stays in zen")
	}
	drive(a, key("esc"))
	if a.state.Zen {
		t.Fatal("second esc leaves zen")
	}
}

func TestZenMouse(t *testing.T) {
	a := bootCol(t, 160, 20, store.State{}, 80)
	drive(a, key("F"))
	before := a.reader.offset
	drive(a, wheel(3, 3, false))
	if a.reader.offset != before+3 {
		t.Fatalf("wheel: %d -> %d", before, a.reader.offset)
	}
	drive(a, click(3, 3), click(3, a.h-1), click(0, 0))
	if !a.state.Zen {
		t.Fatal("clicks changed mode")
	}
}

func TestZenTinyTerminal(t *testing.T) {
	a := bootCol(t, 40, 8, store.State{}, 80)
	drive(a, key("F"))
	_ = a.View()
	drive(a, key("j"), key("l"), key("F"))
	_ = a.View()
	for _, h := range []int{1, 2, 3} {
		b := bootCol(t, 40, h, store.State{Zen: true}, 80)
		_ = b.View()
	}
}
