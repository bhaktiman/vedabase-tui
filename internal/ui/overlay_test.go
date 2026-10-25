package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/store"
)

func plainLines(a *App) []string { return strings.Split(plain(a), "\n") }

// An overlay is drawn over the app, which stays visible around it.
func TestOverlayKeepsScreenAround(t *testing.T) {
	for _, k := range []string{"s", "'", "/", "?"} {
		a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
		before := plainLines(a)
		drive(a, key(k))
		if !a.overlayOpen() {
			t.Fatalf("%s: no overlay", k)
		}
		after := plainLines(a)
		r := a.overlayRect(a.overlayBox())
		if len(after) != a.h || (r.y > 0 && after[0] != before[0]) { // help is full height
			t.Errorf("%s: top bar gone or height %d: %q", k, len(after), after[0])
		}
		for i, l := range strings.Split(a.View().Content, "\n") {
			if w := lipgloss.Width(l); w != a.w {
				t.Errorf("%s: row %d is %d wide", k, i, w)
			}
		}
		row := r.y + r.h/2
		left := ansi.Cut(after[row], 0, r.x)
		if strings.TrimSpace(left) == "" || left != ansi.Cut(before[row], 0, r.x) {
			t.Errorf("%s: left of the box %q, was %q", k, left, ansi.Cut(before[row], 0, r.x))
		}
	}
}

// The theme picker sits over the sidebar, so the reader shows the preview.
func TestThemePickerLeavesReaderText(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	_, reader := a.paneRects()
	before := plainLines(a)
	drive(a, key("T"))
	r := a.overlayRect(a.themeView())
	if r.x+r.w > reader.x {
		t.Fatalf("theme box %+v reaches into the reader at %d", r, reader.x)
	}
	after := plainLines(a)
	for y := reader.y; y < reader.y+reader.h; y++ {
		if got, was := ansi.Cut(after[y], reader.x, a.w), ansi.Cut(before[y], reader.x, a.w); got != was {
			t.Fatalf("reader row %d changed:\n%q\n%q", y, got, was)
		}
	}
	if !strings.Contains(plain(a), "As the embodied soul") {
		t.Fatal("reader text hidden")
	}

	drive(a, key("esc"), key("t"), key("T")) // no sidebar: the box goes to the right edge
	r = a.overlayRect(a.themeView())
	if r.x+r.w != a.w-1 {
		t.Fatalf("theme box %+v, screen %d", r, a.w)
	}
}

// A toast during an overlay shows once, on the bottom row below the box.
func TestOverlayToastOnce(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("s"))
	if !a.picker.visible {
		t.Fatal("no picker")
	}
	clearToast(a)
	a.Toast("hello there", false)
	v := plainLines(a)
	if n := strings.Count(strings.Join(v, "\n"), "hello there"); n != 1 || !strings.Contains(v[len(v)-1], "hello there") {
		t.Fatalf("toast %d times; last row %q", n, v[len(v)-1])
	}
}
