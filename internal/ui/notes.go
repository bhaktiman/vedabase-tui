package ui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/render"
)

// noteEditor is the `a` overlay: a multi-line box on one reference's note.
// It takes every key (see onKey) and never drops the text: Esc, Ctrl-s,
// Ctrl-c and a click outside all save it first.
type noteEditor struct {
	visible bool
	ref     string
	ta      textarea.Model
}

const noteHint = "esc or ctrl-s save · enter new line · empty deletes"

func newNoteArea(pal render.Palette) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 0 // no cap on lines: a long note is never cut off
	st := ta.Styles()
	st.Cursor.Blink = false
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.Text = lipgloss.NewStyle()
	st.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(pal.Dim)
	st.Focused.Placeholder = lipgloss.NewStyle().Foreground(pal.Dim)
	st.Focused.Prompt = lipgloss.NewStyle()
	ta.SetStyles(st)
	ta.Placeholder = "Write a note…"
	return ta
}

// openNote shows the editor on the target item's note.
func (a *App) openNote() tea.Cmd {
	if a.notes == nil {
		return a.Toast("Notes are unavailable", true)
	}
	r := a.target()
	if r == "" {
		return nil
	}
	ta := newNoteArea(a.pal)
	if n, ok := a.notes.Get(r); ok {
		ta.SetValue(n.Text)
	}
	a.noteEd = noteEditor{visible: true, ref: r, ta: ta}
	a.sizeNote()
	return a.noteEd.ta.Focus()
}

// sizeNote fits the text area to the box noteView draws.
func (a *App) sizeNote() {
	bw := max(min(a.w-4, 80), 24)
	a.noteEd.ta.SetWidth(max(bw-4, 1))
	a.noteEd.ta.SetHeight(min(max(a.h-8, 3), 16))
}

// noteKey takes every key while the editor is open.
func (a *App) noteKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.String() {
	case "esc", "ctrl+s", "ctrl+c":
		return a.noteSave()
	}
	var cmd tea.Cmd
	a.noteEd.ta, cmd = a.noteEd.ta.Update(kp)
	return cmd
}

// notePaste puts pasted text into the editor.
func (a *App) notePaste(m tea.PasteMsg) tea.Cmd {
	var cmd tea.Cmd
	a.noteEd.ta, cmd = a.noteEd.ta.Update(m)
	return cmd
}

// noteSave stores the text and closes the editor. When storing fails the
// editor stays open with the text, and the toast says why.
func (a *App) noteSave() tea.Cmd {
	e := &a.noteEd
	text := e.ta.Value()
	_, had := a.notes.Get(e.ref)
	if err := a.notes.Set(e.ref, text, a.now()); err != nil {
		return a.Toast("Couldn't save: "+err.Error(), true)
	}
	r := e.ref
	e.ta.Blur()
	*e = noteEditor{}
	a.noteRev++
	var toast tea.Cmd
	switch {
	case strings.TrimSpace(text) != "":
		toast = a.Toast("Note saved", false)
	case had:
		toast = a.Toast("Note deleted", false)
	}
	if r == a.ref {
		return tea.Batch(a.renderCmd(), toast)
	}
	return toast
}

// FlushNote saves a note still open in the editor, for when the program ends
// some way other than a key.
func (a *App) FlushNote() {
	if a.noteEd.visible && a.notes != nil {
		_ = a.notes.Set(a.noteEd.ref, a.noteEd.ta.Value(), a.now())
	}
}

// noteOf is the note on r, or "".
func (a *App) noteOf(r string) string {
	if a.notes == nil {
		return ""
	}
	n, _ := a.notes.Get(r)
	return n.Text
}

func (a *App) noteView() string {
	a.sizeNote()
	bw := max(min(a.w-4, 80), 24)
	inner := bw - 4
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	title := gradientBold("Note · "+ref.Display(a.noteEd.ref), stops(a.pal)...)
	lines := []string{title, a.noteEd.ta.View(), dim.Render(truncate(noteHint, inner))}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(a.pal.Dim).
		Padding(0, 1).Width(bw).Render(strings.Join(lines, "\n"))
}
