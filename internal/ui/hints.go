package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"vedabase-tui/internal/render"
)

// hintAlphabet is the home row first, so labels are quick to type.
const hintAlphabet = "asdfghjkl;eruiow"

type hint struct {
	label string
	link  render.Link
}

// hintState is the "f" key-label mode over the links on screen.
type hintState struct {
	active bool
	items  []hint
	typed  string
}

// hintLabels gives n links single-letter labels, or two letters each when
// there are more than the alphabet holds (aa, as, ad, …). Links past 256
// get none.
func hintLabels(n int) []string {
	var out []string
	if n <= len(hintAlphabet) {
		for _, c := range hintAlphabet[:n] {
			out = append(out, string(c))
		}
		return out
	}
	for _, c1 := range hintAlphabet {
		for _, c2 := range hintAlphabet {
			out = append(out, string(c1)+string(c2))
		}
	}
	return out[:min(n, len(out))]
}

// startHints labels every link on screen, or toasts when there are none.
func (a *App) startHints() tea.Cmd {
	links := a.reader.VisibleLinks()
	if len(links) == 0 {
		return a.Toast("No links on screen", false)
	}
	labels := hintLabels(len(links))
	a.hints = hintState{active: true}
	for i, l := range links[:len(labels)] {
		a.hints.items = append(a.hints.items, hint{labels[i], l})
	}
	return nil
}

// hintKey takes a key while the labels are shown. Anything that does not
// continue a label cancels and is swallowed.
func (a *App) hintKey(kp tea.KeyPressMsg) tea.Cmd {
	k := kp.String()
	if len(k) != 1 || !strings.Contains(hintAlphabet, k) {
		a.hints = hintState{}
		return nil
	}
	typed := a.hints.typed + k
	matches := 0
	for _, h := range a.hints.items {
		if h.label == typed {
			a.hints = hintState{}
			return a.follow(h.link.Ref)
		}
		if strings.HasPrefix(h.label, typed) {
			matches++
		}
	}
	if matches == 0 {
		a.hints = hintState{}
		return nil
	}
	a.hints.typed = typed
	return nil
}

// drawHints puts the labels over the starts of the links in body, the
// reader's rows before any indent is added.
func (a *App) drawHints(body []string) {
	st := lipgloss.NewStyle().Foreground(a.pal.Accent).Bold(true).Reverse(true)
	for _, h := range a.hints.items {
		if !strings.HasPrefix(h.label, a.hints.typed) {
			continue
		}
		row := h.link.Line - a.reader.Offset()
		if row < 0 || row >= len(body) {
			continue
		}
		lab := h.label[len(a.hints.typed):]
		end := min(h.link.Start+len(lab), a.reader.w)
		line := body[row]
		body[row] = ansi.Cut(line, 0, h.link.Start) + st.Render(lab[:end-h.link.Start]) + ansi.Cut(line, end, a.reader.w)
	}
}

func hintsBar(w int, dim lipgloss.Style) string {
	return padTo(dim.Render("type a label · esc cancel"), w)
}
