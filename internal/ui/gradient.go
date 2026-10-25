package ui

import (
	"image/color"
	"math"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/render"
)

// chapterBarW is the width of the chapter progress bar in the top bar.
const chapterBarW = 12

// stops are the gradient's colours for a palette, so every theme gets its own.
func stops(p render.Palette) []color.Color { return []color.Color{p.Gold, p.Rose, p.Violet} }

// ramp is n colours along the stops. Blend1D returns the stops themselves when
// n is not larger than their number, which would lose the far end of a short
// ramp, so those cases sample the stops evenly instead.
func ramp(n int, st ...color.Color) []color.Color {
	switch {
	case n <= 0 || len(st) == 0:
		return nil
	case n == 1:
		return []color.Color{st[0]}
	case n <= len(st):
		out := make([]color.Color, n)
		for i := range out {
			out[i] = st[i*(len(st)-1)/(n-1)]
		}
		return out
	}
	return lipgloss.Blend1D(n, st...)
}

// gradientText colours s one cell at a time along the stops. A rune and the
// zero-width marks after it share a colour, so the width never changes.
func gradientText(s string, st ...color.Color) string {
	return gradientStyled(s, lipgloss.NewStyle(), st)
}

// gradientBold is gradientText in bold.
func gradientBold(s string, st ...color.Color) string {
	return gradientStyled(s, lipgloss.NewStyle().Bold(true), st)
}

func gradientStyled(s string, base lipgloss.Style, st []color.Color) string {
	var groups []string
	for _, r := range s {
		if n := len(groups); n > 0 && unicode.Is(unicode.Mn, r) {
			groups[n-1] += string(r)
			continue
		}
		groups = append(groups, string(r))
	}
	cols := ramp(len(groups), st...)
	var b strings.Builder
	for i, g := range groups {
		b.WriteString(base.Foreground(cols[i]).Render(g))
	}
	return b.String()
}

// chapterFill is how many of the bar's cells are filled for item i of t.
func chapterFill(i, t int) int {
	if i <= 0 || t <= 0 {
		return 0
	}
	return min(max(int(math.Round(float64(chapterBarW)*float64(i)/float64(t))), 1), chapterBarW)
}

// chapterBar is the progress bar for item i of t: filled cells take their
// colour from their place along the whole bar, the rest are dim.
func (a *App) chapterBar(i, t int) string {
	fill := chapterFill(i, t)
	cols := ramp(chapterBarW, stops(a.pal)...)
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	var b strings.Builder
	for k := 0; k < chapterBarW; k++ {
		if k < fill {
			b.WriteString(lipgloss.NewStyle().Foreground(cols[k]).Render("━"))
		} else {
			b.WriteString(dim.Render("─"))
		}
	}
	return b.String()
}

// thumb is a run of the reader's right edge, in inner rows.
type thumb struct{ start, h int }

// readerThumb is where the scroll marker sits, if the open item overflows.
func (a *App) readerThumb() (thumb, bool) {
	total, off, h := a.reader.Total(), a.reader.Offset(), a.innerH()
	if !a.framed() || !a.hasItem || total <= h {
		return thumb{}, false
	}
	th := max(int(math.Round(float64(h*h)/float64(total))), 1)
	start := int(math.Round(float64(h-th) * float64(off) / float64(total-h)))
	return thumb{min(max(start, 0), h-th), th}, true
}
