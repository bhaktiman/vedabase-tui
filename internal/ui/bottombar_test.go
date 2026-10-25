package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/store"
)

var hintOrder = []string{"/ search", "s save", "e export", "w full width", "t sidebar", "T theme",
	"o web", ": go to", "h/l prev/next", "tab focus", "? help"}

func bottomLine(a *App) string {
	l, _ := a.bottomLayout()
	return l
}

func TestBottomBarHints(t *testing.T) {
	a := bootCol(t, 200, 30, store.State{}, 80)
	line := bottomLine(a)
	txt := ansiRe.ReplaceAllString(line, "")
	last := -1
	for _, h := range hintOrder {
		i := strings.Index(txt, h)
		if i < 0 || i < last {
			t.Fatalf("hint %q missing or out of order in %q", h, txt)
		}
		last = i
	}
	if strings.Contains(txt, "q quit") {
		t.Fatalf("q quit still shown: %q", txt)
	}
	// Plain Dim, as before: the whole hint run is one Dim span, no Gold, no bold.
	hints := strings.Join(hintOrder, " · ")
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	if !strings.Contains(line, dim.Render(hints)) {
		t.Fatalf("hints are not plain Dim: %q", line)
	}
	b := bootCol(t, 200, 30, store.State{Wide: true}, 80)
	if !strings.Contains(plain(b), "w centred") {
		t.Fatal("wide state should say w centred")
	}
}

func TestBottomBarDropsInOrder(t *testing.T) {
	keep := []string{"? help", "/ search", "s save", "e export", "w ", "t sidebar", "T theme",
		"o web", ": go to", "h/l prev/next", "tab focus"}
	sawPartial := false
	for w := 40; w <= 170; w++ {
		a := bootCol(t, w, 30, store.State{}, 80)
		txt := ansiRe.ReplaceAllString(bottomLine(a), "")
		present := make([]bool, len(keep))
		for i, k := range keep {
			present[i] = strings.Contains(txt, k)
		}
		seenMissing := false
		for i, p := range present {
			if !p {
				seenMissing = true
			} else if seenMissing {
				t.Fatalf("w=%d: %q present after a dropped hint: %q", w, keep[i], txt)
			}
		}
		if present[0] && !present[len(keep)-1] {
			sawPartial = true
		}
		// no hint cut mid-word: partial fragments of any hint must not appear
		for _, frag := range []string{"prev/nex ", "tab foc ", "sideba ", "searc ", "expor "} {
			if strings.Contains(txt, frag) {
				t.Fatalf("w=%d: cut hint %q in %q", w, frag, txt)
			}
		}
		if lipgloss.Width(bottomLine(a)) > w {
			t.Fatalf("w=%d: line %d wide", w, lipgloss.Width(bottomLine(a)))
		}
	}
	if !sawPartial {
		t.Fatal("never saw a partial hint set")
	}
}

func TestBottomBarPartClicksStillWork(t *testing.T) {
	for _, w := range []int{160, 70} {
		a := bootCol(t, w, 30, store.State{}, 80)
		_, hits := a.bottomLayout()
		x := -1
		for _, h := range hits {
			if h.key == "3" {
				x = h.x0 + 1
			}
		}
		if x < 0 {
			t.Fatalf("w=%d: no hit", w)
		}
		drive(a, click(x, a.h-1))
		if a.parts.Synonyms {
			t.Fatalf("w=%d: 3 words still on", w)
		}
		drive(a, click(x, a.h-1))
		if !a.parts.Synonyms {
			t.Fatalf("w=%d: 3 words not back", w)
		}
	}
}
