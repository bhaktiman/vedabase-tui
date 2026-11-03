package ui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/unicode/norm"
)

// findMatch is one hit of the page find: the cells [c0, c1) of rendered line.
type findMatch struct{ line, c0, c1 int }

// foldRunes is s lowercased with diacritics stripped, one entry per folded
// rune, each with the index of the rune of s it came from (same folding as
// fold).
func foldRunes(s string) (folded []rune, from []int) {
	for i, r := range []rune(s) {
		for _, f := range norm.NFD.String(string(r)) {
			if !unicode.Is(unicode.Mn, f) {
				folded = append(folded, unicode.ToLower(f))
				from = append(from, i)
			}
		}
	}
	return folded, from
}

// findIn lists the matches of q (already folded) in one plain line.
func findIn(line string, q []rune, n int) []findMatch {
	folded, from := foldRunes(line)
	runes := []rune(line)
	var out []findMatch
	for i := 0; i+len(q) <= len(folded); {
		j := 0
		for j < len(q) && folded[i+j] == q[j] {
			j++
		}
		if j < len(q) {
			i++
			continue
		}
		start, end := from[i], from[i+len(q)-1]+1
		c0 := ansi.StringWidth(string(runes[:start]))
		c1 := ansi.StringWidth(string(runes[:end]))
		if c1 > c0 {
			out = append(out, findMatch{n, c0, c1})
		}
		i += len(q)
	}
	return out
}

// SetFind highlights every match of q on the page (case and diacritics
// ignored) and makes the first one at or after line `from` current. An empty
// q clears the find.
func (r *Reader) SetFind(q string, from int) {
	r.find, r.matches, r.cur = q, nil, 0
	fq, _ := foldRunes(strings.TrimSpace(q))
	if len(fq) > 0 {
		for n, l := range r.lines {
			r.matches = append(r.matches, findIn(ansi.Strip(l), fq, n)...)
		}
	}
	for i, m := range r.matches {
		if m.line >= from {
			r.cur = i
			return
		}
	}
}

// ClearFind drops the highlights.
func (r *Reader) ClearFind() { r.find, r.matches, r.cur = "", nil, 0 }

// FindStep moves the current match by d, wrapping, and scrolls it into view.
func (r *Reader) FindStep(d int) {
	if n := len(r.matches); n > 0 {
		r.cur = ((r.cur+d)%n + n) % n
		r.ShowMatch()
	}
}

// ShowMatch scrolls so the current match is on screen, a few lines from the
// top, unless it already is.
func (r *Reader) ShowMatch() {
	if r.cur >= len(r.matches) {
		return
	}
	if l := r.matches[r.cur].line; l < r.offset || l >= r.offset+r.h {
		r.offset = l - 3
		r.clamp()
	}
}

// findStatus is "3/12" or "no matches".
func (r *Reader) findStatus() string {
	if len(r.matches) == 0 {
		return "no matches"
	}
	return fmt.Sprintf("%d/%d", r.cur+1, len(r.matches))
}

// highlight styles the matches on line n (index into r.lines).
func (r *Reader) highlight(line string, n int) string {
	var out strings.Builder
	prev := 0
	for i, m := range r.matches {
		if m.line != n {
			continue
		}
		st := r.hlOther
		if i == r.cur {
			st = r.hlCur
		}
		out.WriteString(ansi.Cut(line, prev, m.c0))
		out.WriteString(st.Render(ansi.Strip(ansi.Cut(line, m.c0, m.c1))))
		prev = m.c1
	}
	if prev == 0 {
		return line
	}
	out.WriteString(ansi.Cut(line, prev, ansi.StringWidth(line)))
	return out.String()
}

// FindBar is the one-line Ctrl-f prompt at the bottom of the screen.
type FindBar struct {
	visible bool
	input   textinput.Model
	from    int // reader line at the top when the bar opened
}

func NewFindBar() FindBar { return FindBar{input: lineInput("find on the page")} }

func (a *App) startFind() tea.Cmd {
	if !a.hasItem || a.reader.Empty() {
		return nil
	}
	a.findBar.visible = true
	a.findBar.from = a.reader.Offset()
	a.findBar.input.SetValue(a.reader.find)
	return a.findBar.input.Focus()
}

func (a *App) hideFind() {
	a.findBar.visible = false
	a.findBar.input.Blur()
}

func (a *App) clearFind() {
	a.hideFind()
	a.reader.ClearFind()
}

// findKey takes every key while the find prompt is open.
func (a *App) findKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.String() {
	case "esc":
		a.clearFind()
		return nil
	case "enter", "kpenter":
		a.hideFind()
		if strings.TrimSpace(a.reader.find) == "" {
			a.reader.ClearFind()
		}
		return nil
	}
	var cmd tea.Cmd
	a.findBar.input, cmd = a.findBar.input.Update(kp)
	a.reader.SetFind(a.findBar.input.Value(), a.findBar.from)
	a.reader.ShowMatch()
	return cmd
}

// findLine is the bottom bar while a find is being typed or kept.
func (a *App) findLine() string {
	status := lipgloss.NewStyle().Foreground(a.pal.Gold).Render(a.reader.findStatus())
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	if a.findBar.visible {
		in := a.findBar.input
		in.SetWidth(max(a.w-lipgloss.Width(a.reader.findStatus())-12, 1))
		return dim.Render("find: ") + in.View() + "  " + status
	}
	return dim.Render("find: ") + truncate(a.reader.find, max(a.w/2, 1)) + "  " + status +
		dim.Render("  n/N step · Esc clear")
}
