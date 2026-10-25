package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/library"
)

const wheelStep = 3

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

// overlayRect is where overlayView puts a box on the screen.
func (a *App) overlayRect(box string) rect {
	w, h := lipgloss.Width(box), lipgloss.Height(box)
	return rect{max(a.w-w, 0) / 2, max(a.h-h, 0) / 2, w, h}
}

func (a *App) overlayOpen() bool {
	return a.help || a.search.visible || a.picker.visible || a.colview.visible
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
	tree, reader := a.paneRects()
	switch {
	case tree.has(m.X, m.Y):
		if !click {
			a.tree.Scroll(d)
			return nil
		}
		a.focusTree = true
		return a.treeClick(m)
	case reader.has(m.X, m.Y):
		if !click {
			a.reader.offset += d
			a.reader.clamp()
			return nil
		}
		a.focusTree = false
	case click && m.Y == a.h-1:
		return a.barClick(m.X)
	}
	return nil
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
	var box string
	switch {
	case a.search.visible:
		a.search.cdView = ""
		box = a.search.View()
	case a.picker.visible:
		box = a.picker.View()
	default:
		box = a.colview.View()
	}
	r := a.overlayRect(box)
	if !r.has(m.X, m.Y) {
		if click {
			switch {
			case a.search.visible:
				a.search.Close()
			case a.picker.visible:
				a.picker.close()
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
	default:
		if click {
			return a.colview.Click(x, y)
		}
		a.colview.Wheel(wheel)
	}
	return nil
}
