package ui

import (
	"regexp"
	"strings"
	"testing"

	"vedabase-tui/internal/store"
)

var sgrRe = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// reverseFgs are the foregrounds ("r;g;b") of every reverse-video run in s.
func reverseFgs(s string) []string {
	var out []string
	for _, m := range sgrRe.FindAllStringSubmatch(s, -1) {
		ps := strings.Split(m[1], ";")
		rev, fg := false, ""
		for k := 0; k < len(ps); k++ {
			switch {
			case ps[k] == "7":
				rev = true
			case ps[k] == "38" && k+4 < len(ps) && ps[k+1] == "2":
				fg = strings.Join(ps[k+2:k+5], ";")
				k += 4
			}
		}
		if rev && fg != "" {
			out = append(out, fg)
		}
	}
	return out
}

func bootTheme(t *testing.T, theme string) *App {
	t.Helper()
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13", Theme: theme})
	if a.pal.Name != theme {
		t.Fatalf("theme %q not applied: %q", theme, a.pal.Name)
	}
	return a
}

func TestTreeCursorUsesAccent(t *testing.T) {
	a := bootTheme(t, "Monokai")
	drive(a, key("tab")) // focus the tree: the cursor row turns solid
	a.View()             // the tree learns it is focused while a frame is built
	fgs := reverseFgs(a.View().Content)
	if len(fgs) == 0 {
		t.Fatal("no reverse-video cursor row")
	}
	for _, fg := range fgs {
		if !near(fg, "249;38;114") {
			t.Fatalf("cursor colour %s, want Monokai accent 249;38;114 (not gold 253;151;31 or 230;219;116)", fg)
		}
	}
}

func TestFocusedBorderUsesAccent(t *testing.T) {
	a := bootTheme(t, "Dracula")
	lines := strings.Split(a.View().Content, "\n")
	_, rd := a.paneRects()
	var corner cell
	found := false
	cs := cells(lines[rd.y])
	// the reader's top-left corner is the first ╭ at or after its column
	n := 0
	for _, c := range cs {
		if n >= rd.x && c.r == '╭' {
			corner, found = c, true
			break
		}
		n++
	}
	if !found {
		t.Fatal("no reader corner")
	}
	if !near(corner.fg, "189;147;249") {
		t.Fatalf("focused corner %s, want Dracula accent 189;147;249", corner.fg)
	}
}

func TestVerseTextStaysGold(t *testing.T) {
	a := bootTheme(t, "Dracula")
	for _, l := range strings.Split(a.View().Content, "\n") {
		for _, c := range cells(l) {
			if c.r == 'द' {
				if !near(c.fg, "255;184;108") {
					t.Fatalf("verse text %s, want Dracula gold 255;184;108", c.fg)
				}
				return
			}
		}
	}
	t.Fatal("no verse line")
}

func TestGradientUsesThemeGrad(t *testing.T) {
	a := bootTheme(t, "Monokai")
	tr, _ := a.paneRects()
	for _, c := range bottomRow(a, tr) {
		if c.r == 'v' {
			if !near(c.fg, "249;38;114") {
				t.Fatalf("tag starts %s, want Monokai grad start 249;38;114", c.fg)
			}
			return
		}
	}
	t.Fatal("no tag")
}
