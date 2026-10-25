package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/store"
)

func click(x, y int) tea.Msg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func wheel(x, y int, up bool) tea.Msg {
	b := tea.MouseWheelDown
	if up {
		b = tea.MouseWheelUp
	}
	return tea.MouseWheelMsg{X: x, Y: y, Button: b}
}

// treeRowAt is a cell on the tree row showing ref, from the app's own geometry.
func treeRowAt(t *testing.T, a *App, ref string) (x, y int) {
	t.Helper()
	for i, r := range a.tree.rows {
		if r.node.Ref == ref {
			rows := a.treeRows()
			return rows.x + r.depth*2 + 6, rows.y + i - a.tree.offset
		}
	}
	t.Fatalf("no tree row %s", ref)
	return
}

func inReader(a *App) (x, y int) {
	_, r := a.paneRects()
	return r.x + r.w/2, r.y + r.h/2
}

func TestClickTreeLeafOpens(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/1"})
	a.focusTree = true
	x, y := treeRowAt(t, a, "bg/2/13")
	drive(a, click(x, y))
	if a.ref != "bg/2/13" || a.focusTree {
		t.Fatalf("ref=%q focusTree=%v", a.ref, a.focusTree)
	}
	if !strings.Contains(plain(a), "As the embodied soul") {
		t.Fatal("reader did not show the verse")
	}
}

func TestClickTreeContainerToggles(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	x, y := treeRowAt(t, a, "bg/10")
	drive(a, click(x, y))
	if a.tree.Selected().Ref != "bg/10" || a.tree.expanded["bg/10"] {
		t.Fatalf("first click should only select: %q %v", a.tree.Selected().Ref, a.tree.expanded["bg/10"])
	}
	drive(a, click(x, y))
	if !a.tree.expanded["bg/10"] {
		t.Fatal("second click did not expand")
	}
	drive(a, click(x, y))
	if a.tree.expanded["bg/10"] {
		t.Fatal("third click did not collapse")
	}
	// The marker toggles an unselected container at once.
	_, y = treeRowAt(t, a, "bg/2")
	drive(a, click(a.treeRows().x+1*2, y))
	if a.tree.expanded["bg/2"] || a.tree.Selected().Ref != "bg/2" {
		t.Fatalf("marker click: expanded=%v sel=%s", a.tree.expanded["bg/2"], a.tree.Selected().Ref)
	}
}

func TestClickFocusesPane(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	tree, _ := a.paneRects()
	drive(a, click(tree.x+2, tree.y+tree.h-2)) // empty tree area
	if !a.focusTree {
		t.Fatal("tree click did not focus the tree")
	}
	x, y := inReader(a)
	drive(a, click(x, y))
	if a.focusTree {
		t.Fatal("reader click did not focus the reader")
	}
}

func TestWheelScrollsPaneUnderPointer(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 12}, a.renderCmd()())
	x, y := inReader(a)
	before := a.reader.offset
	drive(a, wheel(x, y, false))
	if a.reader.offset != before+3 {
		t.Fatalf("reader offset %d -> %d", before, a.reader.offset)
	}
	drive(a, wheel(x, y, true))
	if a.reader.offset != before {
		t.Fatalf("reader offset after up = %d", a.reader.offset)
	}
	tree, _ := a.paneRects()
	ro, to := a.reader.offset, a.tree.offset
	drive(a, wheel(tree.x+2, tree.y+2, false))
	if a.reader.offset != ro || a.tree.offset != to+3 {
		t.Fatalf("reader %d->%d tree %d->%d", ro, a.reader.offset, to, a.tree.offset)
	}
	if c := a.tree.cursor; c < a.tree.offset || c >= a.tree.offset+a.tree.h {
		t.Fatalf("cursor %d left the view at offset %d", c, a.tree.offset)
	}
}

func TestClickSearchResult(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("/"))
	for _, r := range "eternal" {
		drive(a, key(string(r)))
	}
	drive(a, key("enter"))
	if len(a.search.results) < 2 {
		t.Fatalf("results: %d", len(a.search.results))
	}
	r := a.overlayRect(a.search.View())
	drive(a, click(r.x+4, r.y+searchResultsY+rowLines))
	if a.ref != "bg/2/14" || a.search.visible {
		t.Fatalf("ref=%q visible=%v", a.ref, a.search.visible)
	}
}

func TestClickSearchScopeRerunsQuery(t *testing.T) {
	be := newFake(t)
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("/"))
	for _, r := range "eternal" {
		drive(a, key(string(r)))
	}
	drive(a, key("enter"))
	r := a.overlayRect(a.search.View())
	// "Letters" is the third label; walk the labels to find its column.
	col := searchPadX
	for _, n := range a.search.scopeLabels()[:2] {
		col += len(n) + len(scopeSep)
	}
	drive(a, click(r.x+col+1, r.y+searchScopeY))
	if a.search.scope != 2 {
		t.Fatalf("scope = %d", a.search.scope)
	}
	calls := be.searches()
	if last := calls[len(calls)-1]; last.book != "letters" || last.q != "eternal" {
		t.Fatalf("last search %+v", last)
	}
}

func TestClickPartToggle(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	_, hits := a.bottomLayout()
	var x int
	for _, h := range hits {
		if h.key == "3" {
			x = h.x0 + 1
		}
	}
	line := plain(a)
	rows := strings.Split(line, "\n")
	if !strings.Contains(rows[len(rows)-1][x-1:], "3 words") && x == 0 {
		t.Fatal("no hit for 3 words")
	}
	drive(a, click(x, a.h-1))
	if a.parts.Synonyms {
		t.Fatal("3 words still on")
	}
	drive(a, click(x, a.h-1))
	if !a.parts.Synonyms {
		t.Fatal("3 words not back on")
	}
}

func TestClickOutsideOverlayCloses(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("?"), click(1, 1))
	if a.help {
		t.Fatal("help still open")
	}
	drive(a, key("/"))
	drive(a, click(0, 0))
	if a.search.visible {
		t.Fatal("search still open")
	}
}

func TestClickClosesPromptFirst(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key(":"))
	if !a.gotoBar.visible {
		t.Fatal("goto not open")
	}
	x, y := inReader(a)
	drive(a, click(x, y))
	if a.gotoBar.visible {
		t.Fatal("goto still open after click")
	}
	a.focusTree = true
	drive(a, key("f"))
	if !a.filtering {
		t.Fatal("not filtering")
	}
	drive(a, click(x, y))
	if a.filtering {
		t.Fatal("still filtering after click")
	}
}

func TestClickCollectionsRows(t *testing.T) {
	a, _, _, _ := seeded(t)
	drive(a, key("'"))
	r := a.overlayRect(a.colview.View())
	row := func(i int) tea.Msg { return click(r.x+4, r.y+2+i) }
	drive(a, row(0)) // the selected collection: enter it
	if a.colview.in != "Soul" {
		t.Fatalf("in=%q", a.colview.in)
	}
	drive(a, row(1))
	if a.colview.cursor != 1 || !a.colview.visible {
		t.Fatalf("cursor=%d visible=%v", a.colview.cursor, a.colview.visible)
	}
	drive(a, row(1))
	if a.ref != "bg/2/1" || a.colview.visible {
		t.Fatalf("ref=%q visible=%v", a.ref, a.colview.visible)
	}
}

func TestClickPickerRows(t *testing.T) {
	a, cs, _, _ := seeded(t)
	if _, err := cs.Add("Other", collections.Item{Ref: "bg/2/1", Book: "bg", Label: "x"}); err != nil {
		t.Fatal(err)
	}
	drive(a, key("s"))
	r := a.overlayRect(a.picker.View())
	drive(a, click(r.x+4, r.y+3))
	if a.picker.cursor != 1 || !a.picker.visible {
		t.Fatalf("cursor=%d visible=%v", a.picker.cursor, a.picker.visible)
	}
	drive(a, click(r.x+4, r.y+3))
	if a.picker.visible {
		t.Fatal("second click did not choose")
	}
	if !cs.Contains(a.ref) {
		t.Fatalf("%s not added: %+v", a.ref, cs.List())
	}
}

func TestMouseDisabled(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if a.View().MouseMode != tea.MouseModeNone {
		t.Fatal("mouse mode set although the setting is off")
	}
	a.settings.Mouse = true
	if a.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("mouse mode not set")
	}
}

func TestFocusIndicator(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("tab"))
	v := plain(a)
	if !strings.Contains(v, "● Contents") || strings.Contains(v, "● Reading") {
		t.Fatalf("tree focus:\n%s", v)
	}
	lines := strings.Split(v, "\n")
	if !strings.Contains(lines[1], "Contents") || !strings.Contains(lines[1], "╭") {
		t.Fatalf("top border: %q", lines[1])
	}
	drive(a, key("tab"))
	if v = plain(a); !strings.Contains(v, "● Reading") {
		t.Fatalf("reader focus:\n%s", v)
	}
	if !strings.Contains(strings.Split(v, "\n")[1], "Bg 2.13") {
		t.Fatalf("reader title: %q", strings.Split(v, "\n")[1])
	}
	// With the tree hidden the reader keeps its frame.
	drive(a, key("t"))
	if v = plain(a); !strings.Contains(strings.Split(v, "\n")[1], "╭─ Bg 2.13") {
		t.Fatalf("hidden tree:\n%s", v)
	}
}

func TestTinyTerminalNoBorders(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, tea.WindowSizeMsg{Width: 40, Height: 8})
	v := plain(a)
	if strings.ContainsRune(v, '╭') {
		t.Fatalf("border on a tiny terminal:\n%s", v)
	}
	if !strings.Contains(v, "● Reading") {
		t.Fatalf("no focus name:\n%s", v)
	}
	drive(a, click(3, 3), wheel(3, 3, false))
}

func searchAndOpenFirst(t *testing.T, a *App) {
	t.Helper()
	drive(a, key("/"))
	for _, r := range "eternal" {
		drive(a, key(string(r)))
	}
	drive(a, key("enter"))
	if len(a.search.results) < 2 {
		t.Fatalf("results: %d", len(a.search.results))
	}
	drive(a, key("enter"))
	if a.search.visible {
		t.Fatal("search still visible after opening a result")
	}
}

func TestClickSearchResultAfterReopening(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	searchAndOpenFirst(t, a)
	first := a.ref
	drive(a, key("/"))
	r := a.overlayRect(a.search.View())
	drive(a, click(r.x+4, r.y+searchResultsY+rowLines))
	if a.ref != a.search.results[1].Ref || a.ref == first || a.search.visible {
		t.Fatalf("ref=%q want %q visible=%v", a.ref, a.search.results[1].Ref, a.search.visible)
	}
}

func TestClickSearchResultAfterReopeningAndWheel(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 15}) // two visible rows
	searchAndOpenFirst(t, a)
	drive(a, key("/"), key("down")) // down on the last row loads page 2
	if len(a.search.results) != 3 {
		t.Fatalf("results: %d", len(a.search.results))
	}
	r := a.overlayRect(a.search.View())
	drive(a, wheel(r.x+4, r.y+searchResultsY, false))
	if a.search.top == 0 {
		t.Fatal("wheel did not scroll")
	}
	i := a.search.top + 1
	drive(a, click(r.x+4, r.y+searchResultsY+rowLines))
	if a.ref != a.search.results[i].Ref || a.search.visible {
		t.Fatalf("ref=%q want %q visible=%v", a.ref, a.search.results[i].Ref, a.search.visible)
	}
}

func TestSearchRectMatchesDrawing(t *testing.T) {
	for _, sz := range [][2]int{{120, 40}, {80, 24}, {100, 30}, {200, 60}, {121, 41}, {60, 15}, {160, 50}} {
		a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
		drive(a, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		searchAndOpenFirst(t, a)
		drive(a, key("/"))
		r := a.overlayRect(a.search.View())
		lines := strings.Split(plain(a), "\n")
		for i, l := range lines {
			if strings.Contains(l, "Bg 2.14") {
				if want := r.y + searchResultsY + rowLines; i != want {
					t.Errorf("%v: drawn on row %d, rect says %d", sz, i, want)
				}
				if x := strings.Index(l, "Bg 2.14"); x < r.x {
					t.Errorf("%v: drawn col %d < rect.x %d", sz, x, r.x)
				}
			}
		}
	}
}
