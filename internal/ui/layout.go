package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/library"
)

func (a *App) treeVisible() bool {
	return a.lib != nil && !a.state.TreeHidden && a.w >= minTreeCols
}

func (a *App) treeW() int { return min(max(a.w*30/100, 24), 40) }

func (a *App) bodyH() int { return max(a.h-2, 1) }

// readerW is the reader's width; the tree and a one-column divider come off it.
func (a *App) readerW() int {
	if a.treeVisible() {
		return max(a.w-a.treeW()-1, 1)
	}
	return max(a.w, 1)
}

func (a *App) layout() {
	th := a.bodyH()
	if a.filterShown() {
		th = max(th-1, 1)
	}
	a.tree.SetSize(a.treeW(), th)
	a.reader.SetSize(a.readerW(), a.bodyH())
	a.search.SetSize(a.w, a.h)
	a.picker.SetSize(a.w, a.h)
	a.colview.SetSize(a.w, a.h)
}

// relayout re-renders at once when the reader's width changed.
func (a *App) relayout() tea.Cmd {
	a.layout()
	if a.hasItem && a.renderedW != a.readerW() {
		return a.renderCmd()
	}
	return nil
}

// resize re-renders after the window has been quiet for a moment.
func (a *App) resize(w, h int) tea.Cmd {
	a.w, a.h = w, h
	a.focusTree = a.focusTree && a.treeVisible()
	a.layout()
	if a.hasItem && a.renderedW != a.readerW() {
		a.resizeSeq++
		return a.tick(resizeQuiet, resizeTickMsg{a.resizeSeq})
	}
	return nil
}

func (a *App) View() tea.View {
	v := tea.NewView(a.content())
	v.AltScreen = true
	return v
}

func padTo(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func (a *App) content() string {
	if a.lib == nil {
		return a.startupView()
	}
	switch {
	case a.help:
		return a.overlayView(a.helpView())
	case a.search.visible:
		a.search.cdView = ""
		if a.cd != nil && a.cd.kind == cdSearch {
			a.search.cdView = strings.Join(a.countdownLines(a.w/2), " ")
		}
		return a.overlayView(a.search.View())
	case a.picker.visible:
		return a.overlayView(a.picker.View())
	case a.colview.visible:
		return a.overlayView(a.colview.View())
	}
	var body []string
	if level := a.fadeLevel(); level > 0 {
		body = a.reader.LinesFaded(func(l string) string { return a.fade(l, level) })
	} else {
		body = a.reader.Lines()
	}
	if a.status != "" && !a.hasItem {
		for i, l := range a.statusLines() {
			if i < len(body) {
				body[i] = l
			}
		}
	}
	rw := a.readerW()
	if len(body) > 0 { // bottom-right of the reader
		last := len(body) - 1
		body[last] = a.placeToast(body[last], rw)
	}
	if a.treeVisible() {
		tl := strings.Split(a.tree.View(), "\n")
		if a.filterShown() {
			tl = append(tl, a.filterLine())
		}
		div := lipgloss.NewStyle().Foreground(a.pal.Dim).Render("│")
		for i := range body {
			left := ""
			if i < len(tl) {
				left = tl[i]
			}
			body[i] = padTo(left, a.treeW()) + div + body[i]
		}
	}
	return strings.Join(append(append([]string{a.topBar()}, body...), a.bottomBar()), "\n")
}

// overlayView centres an overlay box on the screen, toasts still at the bottom.
func (a *App) overlayView(box string) string {
	lines := strings.Split(lipgloss.Place(a.w, a.h, lipgloss.Center, lipgloss.Center, box), "\n")
	if len(lines) > 0 {
		last := len(lines) - 1
		lines[last] = a.placeToast(lines[last], a.w)
	}
	return strings.Join(lines, "\n")
}

func (a *App) startupView() string {
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	var line string
	switch {
	case a.libErr != nil:
		line = lipgloss.NewStyle().Foreground(a.pal.Rose).Render("Can't reach vedabase.cc — press r to retry, q to quit")
	case a.progTotal.Load() > 0:
		line = a.startupBar(a.progDone.Load(), a.progTotal.Load())
	default:
		line = dim.Render("Loading contents…")
	}
	return lipgloss.PlaceHorizontal(a.w, lipgloss.Center, line)
}

// statusLines is what replaces the document while there is none: the loader,
// the rate-limit wait, or a plain message.
func (a *App) statusLines() []string {
	w := a.readerW()
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	switch {
	case a.status == loadingStatus && a.fx.Live(Sparkle):
		return append([]string{a.sparkleGlyphs() + " " + dim.Render(loadingStatus), ""}, a.shimmerLines(w)...)
	case a.status == loadingStatus:
		return []string{dim.Render(loadingStatus)}
	}
	rose := lipgloss.NewStyle().Foreground(a.pal.Rose)
	lines := a.countdownLines(w)
	return append(lines, strings.Split(rose.Width(w).Render(a.status), "\n")...)
}

func (a *App) topBar() string {
	if a.node == nil {
		return padTo(lipgloss.NewStyle().Foreground(a.pal.Dim).Render(truncate(a.ref, a.w)), a.w)
	}
	var crumbs []string
	for n := a.node; n != nil; n = n.Parent {
		crumbs = append([]string{n.Label()}, crumbs...)
	}
	pos := ""
	if i, t := library.Position(a.node); t > 0 {
		pos = fmt.Sprintf("%d / %d", i, t)
	}
	left := truncate(strings.Join(crumbs, " › "), max(a.w-len(pos)-2, 1))
	gap := max(a.w-lipgloss.Width(left)-lipgloss.Width(pos), 1)
	return lipgloss.NewStyle().Foreground(a.pal.Bright).Bold(true).Render(left) +
		strings.Repeat(" ", gap) + lipgloss.NewStyle().Foreground(a.pal.Dim).Render(pos)
}

func (a *App) bottomBar() string {
	if a.gotoBar.visible {
		return padTo(a.gotoBar.View(a.w), a.w)
	}
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	on := lipgloss.NewStyle().Foreground(a.pal.Teal)
	off := dim.Strikethrough(true)
	parts := []struct {
		on   bool
		name string
	}{
		{a.parts.Devanagari, "1 dev"}, {a.parts.Verse, "2 verse"}, {a.parts.Synonyms, "3 words"},
		{a.parts.Translation, "4 trans"}, {a.parts.Purport, "5 purport"},
	}
	var ps []string
	plain := 0
	for _, p := range parts {
		st := off
		if p.on {
			st = on
		}
		ps = append(ps, st.Render(p.name))
		plain += len(p.name) + 1
	}
	right := strings.Join(ps, " ")
	if ex := a.exportStatus(); ex != "" {
		right = lipgloss.NewStyle().Foreground(a.pal.Gold).Render(ex) + dim.Render(" · ") + right
		plain += lipgloss.Width(ex) + 3
	}
	hints := "tab focus · h/l prev/next · / search · : go to · ? help · q quit"
	room := a.w - plain - 2
	if room < 10 {
		return padTo(lipgloss.NewStyle().MaxWidth(max(a.w, 1)).Render(right), a.w)
	}
	left := dim.Render(truncate(hints, room))
	return left + strings.Repeat(" ", max(a.w-lipgloss.Width(left)-plain+1, 1)) + right
}
