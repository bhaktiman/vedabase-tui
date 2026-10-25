package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/render"
	"vedabase-tui/internal/store"
)

// A click on the hint line or the bottom border, just below the visible rows,
// must not select the row that is scrolled out of view.
func TestOverlayClickBelowVisibleRowsIgnored(t *testing.T) {
	a, cs, _, _ := seeded(t)
	for i := 0; i < 30; i++ {
		if _, err := cs.Add(fmt.Sprintf("Col%02d", i), collections.Item{Ref: "bg/2/1", Book: "bg", Label: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	vis, _ := listView(a.h, 31, 0)

	drive(a, key("'"))
	if cmd := a.colview.Click(5, 2+vis); cmd != nil || a.colview.cursor != 0 || !a.colview.visible {
		t.Errorf("collections: cursor=%d visible=%v", a.colview.cursor, a.colview.visible)
	}
	drive(a, key("esc"), key("s"))
	if cmd := a.picker.Click(5, 2+vis); cmd != nil || a.picker.cursor != 0 || !a.picker.visible {
		t.Errorf("picker: cursor=%d visible=%v", a.picker.cursor, a.picker.visible)
	}
}

func TestThemeClickBelowVisibleRowsIgnored(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 14}, key("T"))
	vis, _ := listView(a.h, len(render.Themes), a.themes.cursor)
	if vis >= len(render.Themes) {
		t.Fatalf("test needs a scrolling list: vis=%d themes=%d", vis, len(render.Themes))
	}
	before := a.themes.cursor
	a.themeClick(5, 2+vis)
	if a.themes.cursor != before || !a.themes.visible {
		t.Errorf("cursor %d -> %d, visible=%v", before, a.themes.cursor, a.themes.visible)
	}
}
