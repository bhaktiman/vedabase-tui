package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/store"
)

func TestThemePickerLivePreview(t *testing.T) {
	a, sv := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("T"))
	if !a.themes.visible {
		t.Fatal("T did not open the picker")
	}
	drive(a, key("j"))
	if a.pal.Name != "Terminal" {
		t.Fatalf("after j: %q", a.pal.Name)
	}
	drive(a, key("j"))
	if a.pal.Name != "Dracula" {
		t.Fatalf("after jj: %q", a.pal.Name)
	}
	if !strings.Contains(plain(a), "Theme") {
		t.Fatal("picker not drawn")
	}
	drive(a, key("esc"))
	if a.pal.Name != "Vedabase" || a.themes.visible {
		t.Fatalf("after esc: %q visible=%v", a.pal.Name, a.themes.visible)
	}
	drive(a, key("q"))
	for _, s := range sv.calls {
		if s.Theme == "Dracula" {
			t.Fatal("cancelled theme was saved")
		}
	}
}

func TestThemePickerQRestores(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("T"), key("j"), key("j"), key("q"))
	if a.pal.Name != "Vedabase" || a.themes.visible || a.tree.pal.Name != "Vedabase" {
		t.Fatalf("pal=%q tree=%q visible=%v", a.pal.Name, a.tree.pal.Name, a.themes.visible)
	}
}

func TestThemePickerKeep(t *testing.T) {
	be := newFake(t)
	a, sv := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("T"))
	for i := 0; i < 7; i++ {
		drive(a, key("j"))
	}
	drive(a, key("enter"))
	if a.pal.Name != "Nord" || a.themes.visible {
		t.Fatalf("pal=%q visible=%v", a.pal.Name, a.themes.visible)
	}
	if n := len(sv.calls); n == 0 || sv.calls[n-1].Theme != "Nord" {
		t.Fatalf("saved %+v", sv.calls)
	}
	b, _ := boot(t, be, sv.calls[len(sv.calls)-1])
	if b.pal.Name != "Nord" || b.tree.pal.Name != "Nord" {
		t.Fatalf("restart theme = %q", b.pal.Name)
	}
	c, _ := boot(t, be, store.State{Theme: "no such theme"})
	if c.pal.Name != "Vedabase" {
		t.Fatalf("unknown theme gave %q", c.pal.Name)
	}
}

func TestThemePaintsBackground(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if a.View().BackgroundColor == nil {
		t.Fatal("Vedabase should paint")
	}
	drive(a, key("T"), key("j"), key("j"))
	v := a.View()
	if v.BackgroundColor != lipgloss.Color("#282a36") || v.ForegroundColor != lipgloss.Color("#f8f8f2") {
		t.Fatalf("bg=%v fg=%v", v.BackgroundColor, v.ForegroundColor)
	}
	drive(a, key("k"))
	if v := a.View(); a.pal.Name != "Terminal" || v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatalf("%s: bg=%v fg=%v", a.pal.Name, v.BackgroundColor, v.ForegroundColor)
	}
}

func TestThemeReRendersReader(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.reader.offset = 2
	before := strings.Join(a.reader.Lines(), "\n")
	drive(a, key("T"), key("j"), key("j"))
	after := strings.Join(a.reader.Lines(), "\n")
	if before == after {
		t.Fatal("reader lines did not change with the theme")
	}
	if !strings.Contains(after, "38;2;255;184;108") { // Dracula's gold
		t.Fatalf("no Dracula gold in the reader:\n%q", after)
	}
	if a.reader.offset != 2 {
		t.Fatalf("offset = %d", a.reader.offset)
	}
}

func TestThemePickerClick(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("T"))
	r := a.overlayRect(a.themeView())
	drive(a, click(r.x+4, r.y+2+2)) // third row: Dracula
	if a.pal.Name != "Dracula" || !a.themes.visible {
		t.Fatalf("pal=%q visible=%v", a.pal.Name, a.themes.visible)
	}
	drive(a, wheel(r.x+4, r.y+4, false))
	if a.pal.Name != "Solarized Light" { // wheel step is 3 rows
		t.Fatalf("after wheel: %q", a.pal.Name)
	}
	drive(a, click(r.x+4, r.y+2+5)) // already selected: keep
	if a.themes.visible || a.pal.Name != "Solarized Light" {
		t.Fatalf("pal=%q visible=%v", a.pal.Name, a.themes.visible)
	}
	drive(a, key("T"), key("j"))
	drive(a, click(0, 0)) // outside the box: cancel
	if a.themes.visible || a.pal.Name != "Solarized Light" {
		t.Fatalf("outside click: pal=%q visible=%v", a.pal.Name, a.themes.visible)
	}
}

func TestThemeKeyIgnoredInOverlays(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("?"), key("T"))
	if a.themes.visible {
		t.Fatal("T opened over help")
	}
	drive(a, key(":"), key("T"))
	if a.themes.visible || a.gotoBar.input.Value() != "T" {
		t.Fatalf("T leaked from the goto prompt: visible=%v value=%q", a.themes.visible, a.gotoBar.input.Value())
	}
}
