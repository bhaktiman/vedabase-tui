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
	a.tree.SetSize(a.treeW(), a.bodyH())
	a.reader.SetSize(a.readerW(), a.bodyH())
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
	body := a.reader.Lines()
	if a.status != "" && !a.hasItem {
		st := lipgloss.NewStyle().Foreground(a.pal.Dim)
		if a.status != "Loading…" {
			st = lipgloss.NewStyle().Foreground(a.pal.Rose)
		}
		for i, l := range strings.Split(st.Width(a.readerW()).Render(a.status), "\n") {
			if i < len(body) {
				body[i] = l
			}
		}
	}
	rw := a.readerW()
	if t := a.toastView(rw); t != "" && len(body) > 0 { // bottom-right of the reader
		last := len(body) - 1
		keep := max(rw-lipgloss.Width(t)-1, 0)
		body[last] = padTo(lipgloss.NewStyle().MaxWidth(max(keep, 1)).Render(body[last]), keep) + " " + t
	}
	if a.treeVisible() {
		tl := strings.Split(a.tree.View(), "\n")
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

func (a *App) startupView() string {
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	var line string
	switch {
	case a.libErr != nil:
		line = lipgloss.NewStyle().Foreground(a.pal.Rose).Render("Can't reach vedabase.cc — press r to retry, q to quit")
	case a.progTotal.Load() > 0:
		line = dim.Render(fmt.Sprintf("Loading contents… %d%%", a.progDone.Load()*100/a.progTotal.Load()))
	default:
		line = dim.Render("Loading contents…")
	}
	return line
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
	hints := "tab focus · h/l prev/next · t tree · q quit"
	room := a.w - plain - 2
	if room < 10 {
		return padTo(lipgloss.NewStyle().MaxWidth(max(a.w, 1)).Render(right), a.w)
	}
	left := dim.Render(truncate(hints, room))
	return left + strings.Repeat(" ", max(a.w-lipgloss.Width(left)-plain+1, 1)) + right
}
