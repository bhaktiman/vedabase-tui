package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/store"
)

func motion(x, y int) tea.Msg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func release(x, y int) tea.Msg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// Dragging the line between the sidebar and the reader resizes the sidebar,
// and the width is remembered.
func TestDragSidebarEdge(t *testing.T) {
	a, sv := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	w0 := a.treeW() // 36 at 120 columns
	sel := a.tree.cursor
	drive(a, click(w0-1, 10)) // the sidebar's right border
	if !a.dragging || a.tree.cursor != sel {
		t.Fatalf("dragging=%v cursor %d→%d", a.dragging, sel, a.tree.cursor)
	}
	drive(a, motion(49, 12))
	if got := a.treeW(); got != 50 {
		t.Fatalf("width while dragging = %d", got)
	}
	n := len(sv.calls)
	drive(a, release(49, 12))
	if a.dragging || len(sv.calls) != n+1 || sv.calls[n].TreeWidth != 50 {
		t.Fatalf("dragging=%v saves %+v", a.dragging, sv.calls[n:])
	}
	tr, rd := a.paneRects()
	if tr.w != 50 || rd.x != 50 || rd.x+rd.w != a.w {
		t.Fatalf("panes %+v %+v", tr, rd)
	}

	// the reader's left border grabs too; widths stay within limits
	drive(a, click(50, 5), motion(2, 5), release(2, 5))
	if got := a.treeW(); got != treeMinW {
		t.Fatalf("narrowest = %d", got)
	}
	drive(a, click(treeMinW-1, 5), motion(200, 5), release(200, 5))
	if got := a.treeW(); got != a.w-readerMinW {
		t.Fatalf("widest = %d", got)
	}
}

func TestSidebarWidthRestored(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13", TreeWidth: 44})
	if got := a.treeW(); got != 44 {
		t.Fatalf("width = %d", got)
	}
	drive(a, tea.WindowSizeMsg{Width: 60, Height: 40}) // too wide for the window: clamped
	if got := a.treeW(); got != 60-readerMinW {
		t.Fatalf("width in a small window = %d", got)
	}
}

// Elsewhere a click and a motion do what they did before.
func TestNoDragAwayFromEdge(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	w0 := a.treeW()
	drive(a, click(w0+10, 10), motion(60, 10), release(60, 10))
	if a.dragging || a.treeW() != w0 {
		t.Fatalf("dragging=%v width %d", a.dragging, a.treeW())
	}
	drive(a, key("t")) // sidebar hidden: nothing to drag
	drive(a, click(w0-1, 10), motion(60, 10))
	if a.dragging {
		t.Fatal("dragging a hidden sidebar")
	}
}
