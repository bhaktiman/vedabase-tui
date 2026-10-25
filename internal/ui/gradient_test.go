package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/render"
	"vedabase-tui/internal/store"
)

type cell struct {
	r  rune
	fg string // "r;g;b" of the foreground in force, or ""
}

// cells walks s the way a terminal would, tracking only the truecolor
// foreground, and returns one entry per visible rune.
func cells(s string) []cell {
	var out []cell
	fg := ""
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] != 0x1b || i+1 >= len(rs) || rs[i+1] != '[' {
			out = append(out, cell{rs[i], fg})
			continue
		}
		j := i + 2
		for j < len(rs) && (rs[j] < 'A' || rs[j] > 'z') {
			j++
		}
		if j < len(rs) && rs[j] == 'm' {
			ps := strings.Split(string(rs[i+2:j]), ";")
			for k := 0; k < len(ps); k++ {
				switch {
				case ps[k] == "" || ps[k] == "0" || ps[k] == "39":
					fg = ""
				case ps[k] == "38" && k+4 < len(ps) && ps[k+1] == "2":
					fg = strings.Join(ps[k+2:k+5], ";")
					k += 4
				}
			}
		}
		i = j
	}
	return out
}

func rgb(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("%d;%d;%d", r>>8, g>>8, b>>8)
}

// near reports whether two "r;g;b" colours are within 2 per channel (the
// CIELAB round trip is not exact).
func near(a, b string) bool {
	pa, pb := strings.Split(a, ";"), strings.Split(b, ";")
	if len(pa) != 3 || len(pb) != 3 {
		return false
	}
	for i := range pa {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x-y > 2 || y-x > 2 {
			return false
		}
	}
	return true
}

func distinct(cs []cell) int {
	seen := map[string]bool{}
	for _, c := range cs {
		if c.fg != "" {
			seen[c.fg] = true
		}
	}
	return len(seen)
}

func TestGradientText(t *testing.T) {
	stops := []color.Color{lipgloss.Color("#e8a95b"), lipgloss.Color("#e07a8f"), lipgloss.Color("#b48ee8")}
	g := gradientText("abcdefghij", stops...)
	cs := cells(g)
	if len(cs) != 10 {
		t.Fatalf("%d cells in %q", len(cs), g)
	}
	for i, c := range cs {
		if c.fg == "" {
			t.Fatalf("cell %d uncoloured", i)
		}
	}
	if !near(cs[0].fg, rgb(stops[0])) || !near(cs[9].fg, rgb(stops[2])) {
		t.Fatalf("ends %s %s, want %s %s", cs[0].fg, cs[9].fg, rgb(stops[0]), rgb(stops[2]))
	}
	if distinct(cs) < 8 {
		t.Fatalf("only %d distinct colours", distinct(cs))
	}

	const s = "↗ vedabase.cc ॐ ṛṣi"
	g = gradientText(s, stops...)
	if lipgloss.Width(g) != lipgloss.Width(s) {
		t.Fatalf("width %d, want %d", lipgloss.Width(g), lipgloss.Width(s))
	}
	if got := ansiRe.ReplaceAllString(g, ""); got != s {
		t.Fatalf("text %q, want %q", got, s)
	}
	if gradientText("", stops...) != "" {
		t.Fatal("empty string not empty")
	}
	one := cells(gradientText("x", stops...))
	if len(one) != 1 || !near(one[0].fg, rgb(stops[0])) {
		t.Fatalf("single cell %+v", one)
	}
	if !strings.Contains(gradientBold("hi", stops...), "1") {
		t.Fatal("bold variant not bold")
	}
}

func bottomRow(a *App, pane rect) []cell {
	lines := strings.Split(a.View().Content, "\n")
	return cells(lines[pane.y+pane.h-1])
}

func TestSiteTagGradient(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	tr, _ := a.paneRects()
	row := bottomRow(a, tr)
	var tag []cell
	for i, c := range row {
		if c.r == 'v' && i+len("vedabase.cc") <= len(row) {
			tag = row[i : i+len("vedabase.cc")]
			break
		}
	}
	if string(runesOf(tag)) != "vedabase.cc" {
		t.Fatalf("tag not found in %q", string(runesOf(row)))
	}
	if distinct(tag) < 3 {
		t.Fatalf("tag has %d distinct colours", distinct(tag))
	}
}

func runesOf(cs []cell) []rune {
	var r []rune
	for _, c := range cs {
		r = append(r, c.r)
	}
	return r
}

func TestChapterBar(t *testing.T) {
	count := func(a *App) (filled, empty int) {
		for _, c := range []rune(ansiRe.ReplaceAllString(a.topBar(), "")) {
			switch c {
			case '━':
				filled++
			case '─':
				empty++
			}
		}
		return
	}
	for _, c := range []struct{ i, t, want int }{{1, 72, 1}, {13, 72, 2}, {36, 72, 6}, {72, 72, 12}, {0, 72, 0}, {1, 2000, 1}} {
		if got := chapterFill(c.i, c.t); got != c.want {
			t.Errorf("chapterFill(%d, %d) = %d, want %d", c.i, c.t, got, c.want)
		}
	}
	// The sample library has two verses in chapter 2: 1 / 2 fills half, 2 / 2 all.
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/1"})
	if f, e := count(a); f != 6 || e != 6 {
		t.Fatalf("1 / 2: filled %d empty %d\n%s", f, e, a.topBar())
	}
	a, _ = boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if f, e := count(a); f != 12 || e != 0 {
		t.Fatalf("2 / 2: filled %d empty %d", f, e)
	}
	// A short fill shows only the gold end of the full-width gradient.
	var bar []cell
	for _, c := range cells(a.chapterBar(1, 72)) {
		if c.r == '━' {
			bar = append(bar, c)
		}
	}
	if len(bar) != 1 || !near(bar[0].fg, rgb(a.pal.Gold)) {
		t.Fatalf("one-cell fill %+v, want gold %s", bar, rgb(a.pal.Gold))
	}
	// The room rule: the crumb is cut to 20 cells before the bar goes. Pos
	// "2 / 2" is 5 cells, so the bar needs 20+1+12+1+6 plus a gap cell.
	need := 20 + 1 + chapterBarW + 1 + lipgloss.Width("2 / 2") + 1
	a.w = need
	if f, _ := count(a); f == 0 {
		t.Fatalf("bar dropped at width %d, where it fits", need)
	}
	a.w = need - 1
	if f, e := count(a); f+e != 0 {
		t.Fatalf("bar kept at width %d, below the rule", need-1)
	}
	if got := lipgloss.Width(a.topBar()); got != a.w {
		t.Fatalf("top bar is %d wide at %d", got, a.w)
	}
}

func TestReaderScrollThumb(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	long := make([]string, 200)
	for i := range long {
		long[i] = fmt.Sprintf("line %d", i)
	}
	a.reader.SetContent(strings.Join(long, "\n"), false)
	edge := func() (marks []int) {
		lines := strings.Split(plain(a), "\n")
		for i := 0; i < a.innerH(); i++ {
			rs := []rune(lines[2+i])
			if rs[len(rs)-1] == '┃' {
				marks = append(marks, i)
			}
		}
		return
	}
	h := max(1, int(float64(a.innerH()*a.innerH())/200+0.5))
	m := edge()
	if len(m) != h || m[0] != 0 {
		t.Fatalf("top: marks %v, want %d rows from 0", m, h)
	}
	drive(a, key("G"))
	m = edge()
	if len(m) != h || m[len(m)-1] != a.innerH()-1 {
		t.Fatalf("bottom: marks %v, want %d rows ending at %d", m, h, a.innerH()-1)
	}
	// the thumb is coloured by row, the rest of the edge is the border colour
	lines := strings.Split(a.View().Content, "\n")
	var cols []cell
	for i := 0; i < a.innerH(); i++ {
		cs := cells(lines[2+i])
		cols = append(cols, cs[len(cs)-1])
	}
	thumb := map[string]bool{}
	for _, c := range cols {
		if c.r == '┃' {
			thumb[c.fg] = true
		} else if !near(c.fg, rgb(a.pal.Gold)) && !near(c.fg, rgb(a.pal.Dim)) {
			t.Fatalf("plain edge cell %+v", c)
		}
	}
	if len(thumb) < 2 {
		t.Fatalf("thumb rows share one colour: %v", thumb)
	}
	a.reader.SetContent("short\ntext", false)
	if m := edge(); len(m) != 0 {
		t.Fatalf("thumb on a short item: %v", m)
	}
}

func TestGradientFollowsTheme(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	var dracula render.Palette
	for _, p := range render.Themes {
		if p.Name == "Dracula" {
			dracula = p
		}
	}
	a.setPalette(dracula)
	tr, _ := a.paneRects()
	for _, c := range bottomRow(a, tr) {
		if c.r == 'v' {
			if !near(c.fg, "255;184;108") {
				t.Fatalf("tag starts %s, want Dracula gold 255;184;108", c.fg)
			}
			return
		}
	}
	t.Fatal("no tag")
}

func TestBordersStayPlain(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	lines := strings.Split(a.View().Content, "\n")
	tr, rd := a.paneRects()
	focus := func(focused bool) string {
		if focused {
			return rgb(a.pal.Gold)
		}
		return rgb(a.pal.Dim)
	}
	for _, p := range []struct {
		name string
		r    rect
		want string
	}{{"tree", tr, focus(a.focusTree)}, {"reader", rd, focus(!a.focusTree)}} {
		check := func(cs []cell, what string) {
			for _, c := range cs {
				if !near(c.fg, p.want) {
					t.Errorf("%s %s: %q coloured %s, want %s", p.name, what, c.r, c.fg, p.want)
				}
			}
		}
		for y := p.r.y; y < p.r.y+p.r.h; y++ {
			cs := cells(lines[y])
			if p.r.x < len(cs) {
				check(cs[p.r.x:p.r.x+1], "left edge")
			}
		}
		var top []cell
		for _, c := range cells(lines[p.r.y])[p.r.x : p.r.x+p.r.w] {
			if c.r == '╭' || c.r == '─' {
				top = append(top, c)
			}
		}
		check(top, "top edge")
	}
}
