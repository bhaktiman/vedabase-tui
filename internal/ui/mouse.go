package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/library"
)

const (
	wheelStep   = 3
	treeStep    = 4                      // columns per < or >
	doubleClick = 400 * time.Millisecond // two presses on the sidebar edge this close reset its width
)

type rect struct{ x, y, w, h int }

func (r rect) has(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

// paneRects are the tree's and the reader's outer rectangles (the border
// included). The tree's is empty when it is hidden. Row 0 is the top bar.
func (a *App) paneRects() (tree, reader rect) {
	reader = rect{0, 1, a.readerOuterW(), a.bodyH()}
	if a.treeVisible() {
		tree = rect{0, 1, a.treeW(), a.bodyH()}
		reader.x = a.treeW()
		if !a.framed() {
			reader.x++ // the divider column
		}
	}
	return tree, reader
}

// treeRows is where the tree's rows are drawn: its first row and how many.
func (a *App) treeRows() rect {
	t, _ := a.paneRects()
	b := a.bw()
	return rect{t.x + b, t.y + b, a.treeInnerW(), a.tree.h}
}

// overlayRect is where overlayView puts a box on the screen: centred, except
// the theme picker, which keeps off the text so the preview shows on it. It
// goes over the sidebar, or to the right edge when there is none.
func (a *App) overlayRect(box string) rect {
	w, h := lipgloss.Width(box), lipgloss.Height(box)
	r := rect{max(a.w-w, 0) / 2, max(a.h-h, 0) / 2, w, h}
	if a.themes.visible && !a.help && !a.search.visible && !a.picker.visible && !a.colview.visible {
		r.x = max(a.w-w-1, 0)
		if a.treeVisible() {
			r.x = max(a.treeW()-w, 0) / 2
		}
	}
	return r
}

func (a *App) overlayOpen() bool {
	return a.help || a.search.visible || a.picker.visible || a.colview.visible || a.themes.visible
}

func (a *App) onMouse(m tea.Mouse, click bool) tea.Cmd {
	if a.lib == nil {
		return nil
	}
	if click {
		a.fx.FinishAll()
		if m.Button != tea.MouseLeft {
			return nil
		}
		// A prompt being edited gives way to the click.
		switch {
		case a.gotoBar.visible:
			a.gotoBar.hide()
		case a.filtering:
			a.filtering = false // keep the filter: the click may be on a match
			a.filterIn.Blur()
			a.layout()
		}
	} else if m.Button != tea.MouseWheelUp && m.Button != tea.MouseWheelDown {
		return nil
	}
	d := wheelStep
	if m.Button == tea.MouseWheelUp {
		d = -wheelStep
	}
	if a.overlayOpen() {
		return a.overlayMouse(m, click, d)
	}
	if a.state.Zen { // only the text: the wheel scrolls it, clicks do nothing
		if !click {
			a.reader.offset += d
			a.reader.clamp()
		}
		return nil
	}
	if click && a.onTreeEdge(m) {
		now := a.now()
		if !a.edgeClick.IsZero() && now.Sub(a.edgeClick) < doubleClick {
			a.edgeClick = time.Time{}
			a.state.TreeWidth = 0 // a double-click goes back to the automatic width
			a.persist()
			return a.relayout()
		}
		a.edgeClick = now
		a.dragging, a.dragOff = true, a.treeW()-m.X
		return nil
	}
	tree, reader := a.paneRects()
	switch {
	case tree.has(m.X, m.Y):
		if !click {
			a.tree.Scroll(d)
			return nil
		}
		a.focusTree = true
		if tag, ok := a.siteTagRect(); ok && tag.has(m.X, m.Y) {
			return a.openWeb(webHome)
		}
		return a.treeClick(m)
	case reader.has(m.X, m.Y):
		if !click {
			a.reader.offset += d
			a.reader.clamp()
			return nil
		}
		a.focusTree = false
		if a.webButtonOn() && a.webButtonRect().has(m.X, m.Y) {
			return a.openItemWeb()
		}
	case click && m.Y == a.h-1:
		return a.barClick(m.X)
	}
	return nil
}

// onTreeEdge reports whether m is on the line between the sidebar and the
// reader: the sidebar's right border or the reader's left one (the divider
// when unframed).
func (a *App) onTreeEdge(m tea.Mouse) bool {
	if !a.treeVisible() || m.Y < 1 || m.Y > a.bodyH() {
		return false
	}
	return m.X == a.treeW()-1 || m.X == a.treeW()
}

// drag follows a held sidebar edge: the sidebar ends at the pointer. The text
// is re-rendered once the pointer rests, and the width saved on release.
func (a *App) drag(m tea.Mouse, released bool) tea.Cmd {
	if !a.dragging {
		return nil
	}
	if tw := clampTreeW(m.X+a.dragOff, a.w); tw != a.treeW() {
		a.state.TreeWidth = tw
		a.edgeClick = time.Time{} // a drag, not the first half of a double-click
		a.layout()
	}
	if released {
		a.dragging = false
		a.persist()
		return a.relayout()
	}
	if a.hasItem && a.renderedW != a.textW() {
		a.resizeSeq++
		return a.tick(resizeQuiet, resizeTickMsg{a.resizeSeq})
	}
	return nil
}

// resizeTree is < and >: the sidebar narrower or wider by treeStep columns.
func (a *App) resizeTree(k string) tea.Cmd {
	if !a.treeVisible() {
		return nil
	}
	d := treeStep
	if k == "<" {
		d = -treeStep
	}
	a.state.TreeWidth = clampTreeW(a.treeW()+d, a.w)
	a.persist()
	return tea.Batch(a.relayout(), a.Toast(fmt.Sprintf("Sidebar %d columns", a.state.TreeWidth), false))
}

func (a *App) treeClick(m tea.Mouse) tea.Cmd {
	rows := a.treeRows()
	i := m.Y - rows.y
	if i < 0 || i >= rows.h {
		return nil
	}
	idx := a.tree.offset + i
	onMarker := false
	var n *library.Node
	if idx < len(a.tree.rows) {
		n = a.tree.rows[idx].node
		depth := a.tree.rows[idx].depth
		x := m.X - rows.x
		onMarker = !a.tree.rows[idx].node.Leaf && x >= depth*2 && x < depth*2+2
	}
	return a.treeUpdate(n, func(t Tree) (Tree, tea.Cmd) { return t.Click(idx, onMarker) })
}

// barClick toggles the part whose label is at column x of the bottom bar.
func (a *App) barClick(x int) tea.Cmd {
	_, hits := a.bottomLayout()
	for _, h := range hits {
		if x >= h.x0 && x < h.x1 {
			return a.togglePart(h.key)
		}
	}
	return nil
}

// overlayMouse routes a mouse event to the overlay that is open.
func (a *App) overlayMouse(m tea.Mouse, click bool, wheel int) tea.Cmd {
	if a.help {
		if click {
			a.help = false
		}
		return nil
	}
	r := a.overlayRect(a.overlayBox())
	if !r.has(m.X, m.Y) {
		if click {
			switch {
			case a.search.visible:
				a.search.Close()
			case a.picker.visible:
				a.picker.close()
			case a.themes.visible:
				return a.themeCancel()
			default:
				a.colview.close()
			}
		}
		return nil
	}
	x, y := m.X-r.x, m.Y-r.y
	switch {
	case a.search.visible:
		if click {
			return a.search.Click(x, y)
		}
		return a.search.Wheel(wheel)
	case a.picker.visible:
		if click {
			return a.picker.Click(x, y)
		}
		a.picker.Wheel(wheel)
	case a.themes.visible:
		if click {
			return a.themeClick(x, y)
		}
		return a.themeMove(a.themes.cursor + wheel)
	default:
		if click {
			return a.colview.Click(x, y)
		}
		a.colview.Wheel(wheel)
	}
	return nil
}
