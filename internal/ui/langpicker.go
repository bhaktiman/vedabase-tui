package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/library"
)

// langPicker is the `L` overlay: choose the language the library is read in.
type langPicker struct {
	visible bool
	cursor  int
}

func (a *App) openLangs() {
	a.langs = langPicker{visible: true}
	for i, l := range langNames {
		if l.code == a.lang {
			a.langs.cursor = i
		}
	}
}

// langPick closes the picker and switches to row i; the current language
// just closes it.
func (a *App) langPick(i int) tea.Cmd {
	a.langs.visible = false
	if i < 0 || i >= len(langNames) {
		return nil
	}
	return a.SetLang(langNames[i].code)
}

func (a *App) langKey(kp tea.KeyPressMsg) tea.Cmd {
	switch a.seq.Resolve(kp) {
	case "esc", "q":
		a.langs.visible = false
	case "enter":
		return a.langPick(a.langs.cursor)
	case "j", "down":
		a.langs.cursor = min(a.langs.cursor+1, len(langNames)-1)
	case "k", "up":
		a.langs.cursor = max(a.langs.cursor-1, 0)
	}
	return nil
}

// langClick handles a left click at x, y inside the box: a row picks it.
func (a *App) langClick(x, y int) tea.Cmd {
	vis, top := listView(a.h, len(langNames), a.langs.cursor)
	i := top + y - 2
	if y < 2 || y-2 >= vis || i >= len(langNames) {
		return nil
	}
	return a.langPick(i)
}

func (a *App) langView() string {
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	fg := lipgloss.NewStyle().Foreground(a.pal.Fg)
	rows := make([]string, len(langNames))
	for i, l := range langNames {
		rows[i] = fg.Render(l.name)
		if l.code == a.lang {
			rows[i] += dim.Render("  ·  current")
		}
	}
	return listBox(a.pal, themeBoxW+4, a.h, "Language", rows, a.langs.cursor, "", "enter pick · esc cancel")
}

// langTagLabel is the sidebar's top-edge button: the code and a drop arrow.
func (a *App) langTagLabel() string { return strings.ToUpper(a.lang) + " ▾" }

// langTagOn reports whether the sidebar's top border has room for the tag.
func (a *App) langTagOn() bool {
	if !a.framed() || !a.treeVisible() {
		return false
	}
	_, ok := titleRoomFor(a.treeW(), lipgloss.Width("Contents"), btnSpanOf(lipgloss.Width(a.langTagLabel())))
	return ok
}

// langTagRect is where the tag sits on the screen.
func (a *App) langTagRect() (rect, bool) {
	if !a.langTagOn() {
		return rect{}, false
	}
	t, _ := a.paneRects()
	w := lipgloss.Width(a.langTagLabel())
	return rect{t.x + t.w - 3 - w, t.y, w, 1}, true
}

// readerTag marks a letter or transcript read under another language: it
// stays in English.
func (a *App) readerTag() string {
	if a.hasItem && a.lang != "en" && isEnglishOnly(a.ref) {
		return " EN"
	}
	return ""
}

// treeNote is what follows a book's name in the sidebar when the book does
// not exist in the current language.
func (a *App) treeNote(n *library.Node) string {
	if n.Parent != nil || n.Ref == library.CollectionsRef || a.available(n.Ref) {
		return ""
	}
	return " · not in " + strings.ToUpper(a.lang)
}
