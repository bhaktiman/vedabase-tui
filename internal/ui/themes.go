package ui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/render"
)

// themePicker is the `T` overlay. Moving the cursor applies a theme at once;
// Enter keeps it, Esc puts back the one that was active when it opened.
type themePicker struct {
	visible bool
	cursor  int
	orig    render.Palette
}

// themeInput colours a text input from the palette.
func themeInput(in *textinput.Model, p render.Palette) {
	st := in.Styles()
	st.Focused.Text = lipgloss.NewStyle().Foreground(p.Fg)
	st.Blurred.Text = lipgloss.NewStyle().Foreground(p.Dim)
	st.Focused.Placeholder = lipgloss.NewStyle().Foreground(p.Dim)
	st.Blurred.Placeholder = st.Focused.Placeholder
	st.Cursor.Color = p.Accent
	in.SetStyles(st)
}

// setPalette switches the whole app to p and re-renders the open item.
func (a *App) setPalette(p render.Palette) tea.Cmd {
	a.pal = p
	a.tree.pal = p
	a.search.setPalette(p)
	a.picker.pal = p
	a.colview.pal = p
	for _, in := range []*textinput.Model{&a.search.input, &a.picker.input, &a.colview.input, &a.filterIn, &a.gotoBar.input} {
		themeInput(in, p)
	}
	a.rend.SetPalette(p)
	return a.renderCmd()
}

func (a *App) openThemes() {
	a.themes = themePicker{visible: true, orig: a.pal}
	for i, p := range render.Themes {
		if p.Name == a.pal.Name {
			a.themes.cursor = i
		}
	}
}

// themeMove puts the cursor on row i and applies that theme.
func (a *App) themeMove(i int) tea.Cmd {
	i = max(min(i, len(render.Themes)-1), 0)
	if i == a.themes.cursor {
		return nil
	}
	a.themes.cursor = i
	return a.setPalette(render.Themes[i])
}

func (a *App) themeKeep() tea.Cmd {
	a.themes.visible = false
	a.state.Theme = a.pal.Name
	if a.save != nil {
		_ = a.save(a.State())
	}
	return nil
}

func (a *App) themeCancel() tea.Cmd {
	a.themes.visible = false
	if a.pal.Name == a.themes.orig.Name {
		return nil
	}
	return a.setPalette(a.themes.orig)
}

func (a *App) themeKey(kp tea.KeyPressMsg) tea.Cmd {
	switch a.seq.Resolve(kp) {
	case "esc", "q":
		return a.themeCancel()
	case "enter":
		return a.themeKeep()
	case "j", "down":
		return a.themeMove(a.themes.cursor + 1)
	case "k", "up":
		return a.themeMove(a.themes.cursor - 1)
	}
	return nil
}

// themeClick handles a left click at x, y inside the box: a row is applied,
// and the applied row again keeps it.
func (a *App) themeClick(x, y int) tea.Cmd {
	vis, top := listView(a.h, len(render.Themes), a.themes.cursor)
	i := top + y - 2
	if y < 2 || y-2 >= vis || i >= len(render.Themes) {
		return nil
	}
	if i == a.themes.cursor {
		return a.themeKeep()
	}
	return a.themeMove(i)
}

func (a *App) themeView() string {
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	fg := lipgloss.NewStyle().Foreground(a.pal.Fg)
	rows := make([]string, len(render.Themes))
	for i, p := range render.Themes {
		rows[i] = fg.Render(p.Name)
		if p.Name == a.themes.orig.Name {
			rows[i] += dim.Render("  ·  current")
		}
	}
	return listBox(a.pal, a.w, a.h, "Theme", rows, a.themes.cursor, "", "j/k move · enter keep · esc cancel")
}
