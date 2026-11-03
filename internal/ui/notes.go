package ui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/notes"
	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/render"
)

// noteEditor is the `a` overlay: a multi-line box on one reference's note.
// It takes every key (see onKey) and never drops the text: Esc, Ctrl-s,
// Ctrl-c and a click outside all save it first.
type noteEditor struct {
	visible bool
	ref     string
	orig    string // the text the editor was opened on; a save is checked against it
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
	// Catch up with other windows first, so the prefill is the file's text.
	var toast tea.Cmd
	if err := a.notes.Refresh(); err != nil {
		toast = a.Toast("Couldn't read notes: "+err.Error(), true)
	}
	ta := newNoteArea(a.pal)
	n, _ := a.notes.Get(r)
	ta.SetValue(n.Text)
	a.noteEd = noteEditor{visible: true, ref: r, orig: n.Text, ta: ta}
	a.sizeNote()
	return tea.Batch(a.noteEd.ta.Focus(), toast)
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
// editor stays open with the text, and the toast says why. When the note
// changed in another window since the editor opened, the editor stays open
// and the next save overwrites it.
func (a *App) noteSave() tea.Cmd {
	e := &a.noteEd
	text := e.ta.Value()
	r := e.ref
	closeEd := func() {
		e.ta.Blur()
		*e = noteEditor{}
	}
	if text == e.orig {
		closeEd()
		return nil
	}
	before, had := a.notes.Get(r)
	if err := a.notes.SetIf(r, e.orig, text, a.now()); err != nil {
		if errors.Is(err, notes.ErrConflict) {
			cur, _ := a.notes.Get(r)
			e.orig = cur.Text
			return a.Toast("This note changed in another window — Esc again saves yours over it", true)
		}
		return a.Toast("Couldn't save: "+err.Error(), true)
	}
	closeEd()
	after, has := a.notes.Get(r)
	changed := had != has || before.Text != after.Text
	if changed && r == a.ref {
		a.noteRev++
	}
	var toast tea.Cmd
	switch {
	case strings.TrimSpace(text) != "":
		toast = a.Toast("Note saved", false)
	case had:
		toast = a.Toast("Note deleted", false)
	}
	if changed && r == a.ref {
		return tea.Batch(a.renderCmd(), toast)
	}
	return toast
}

// OpenNote is the note still open in the editor: its reference and text.
func (a *App) OpenNote() (ref, text string, ok bool) {
	if !a.noteEd.visible {
		return "", "", false
	}
	return a.noteEd.ref, a.noteEd.ta.Value(), true
}

// FlushNote saves a note still open in the editor, for when the program ends
// some way other than a key. Nothing open or nothing changed is nil; a failed
// save (ErrConflict included) is returned, and OpenNote still has the text.
func (a *App) FlushNote() error {
	if !a.noteEd.visible || a.notes == nil {
		return nil
	}
	text := a.noteEd.ta.Value()
	if text == a.noteEd.orig {
		return nil
	}
	return a.notes.SetIf(a.noteEd.ref, a.noteEd.orig, text, a.now())
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
