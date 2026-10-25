package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/ref"
)

// lineInput is a one-line text box that keeps no timers (steady cursor).
func lineInput(placeholder string) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	st := in.Styles()
	st.Cursor.Blink = false
	in.SetStyles(st)
	return in
}

// Goto is the one-line ":" prompt at the bottom of the screen.
type Goto struct {
	visible bool
	input   textinput.Model
}

func NewGoto() Goto { return Goto{input: lineInput("bg 2.13 · sb 1.2.6 · cc madhya 9.74")} }

func (g *Goto) Show() tea.Cmd {
	g.visible = true
	g.input.SetValue("")
	return g.input.Focus()
}

func (g *Goto) hide() {
	g.visible = false
	g.input.Blur()
}

func (g Goto) View(w int) string {
	g.input.SetWidth(max(w-3, 1))
	return ": " + g.input.View()
}

// onKey takes every key while the prompt is open.
func (a *App) gotoKey(kp tea.KeyPressMsg) tea.Cmd {
	g := &a.gotoBar
	switch kp.String() {
	case "esc":
		g.hide()
		return nil
	case "enter", "kpenter":
		in := strings.TrimSpace(g.input.Value())
		g.hide()
		r, err := ref.ParseGoto(in)
		if err != nil {
			return a.Toast(err.Error(), true)
		}
		if a.lib.Find(r) == nil {
			return a.Toast("No "+in+" in the library", true)
		}
		a.focusTree = false
		return a.Open(r)
	}
	var cmd tea.Cmd
	g.input, cmd = g.input.Update(kp)
	return cmd
}

// filterShown reports whether the filter line is drawn under the tree.
func (a *App) filterShown() bool { return a.filtering || a.tree.filter != "" }

func (a *App) startFilter() tea.Cmd {
	a.filtering = true
	a.filterIn.SetValue(a.tree.filter)
	a.layout()
	return a.filterIn.Focus()
}

// filterKey takes every key while the filter line is being edited.
func (a *App) filterKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.String() {
	case "esc":
		a.filtering = false
		a.filterIn.Blur()
		a.tree.SetFilter("")
		a.layout()
		return nil
	case "enter", "kpenter":
		a.filtering = false
		a.filterIn.Blur()
		a.layout()
		return nil
	}
	var cmd tea.Cmd
	a.filterIn, cmd = a.filterIn.Update(kp)
	a.tree.SetFilter(a.filterIn.Value())
	return cmd
}

func (a *App) filterLine() string {
	in := a.filterIn
	in.SetWidth(max(a.treeInnerW()-2, 1))
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	if !a.filtering {
		return dim.Render("f ") + truncate(a.tree.filter, max(a.treeInnerW()-2, 1))
	}
	return dim.Render("f ") + in.View()
}
