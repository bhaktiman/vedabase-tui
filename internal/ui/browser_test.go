package ui

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"reflect"
	"strings"
	"testing"

	"vedabase-tui/internal/store"
)

func fakeOpener(a *App) *[]string {
	var got []string
	a.opener = func(u string) error { got = append(got, u); return nil }
	return &got
}

func TestOpenKeyOpensItemURL(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	got := fakeOpener(a)
	drive(a, key("o"))
	if !reflect.DeepEqual(*got, []string{a.item.URL}) || a.item.URL == "" {
		t.Fatalf("opened %v, item url %q", *got, a.item.URL)
	}
	if toastText(a) != "Opened on vedabase.cc" {
		t.Fatalf("toast %q", toastText(a))
	}
}

func TestOpenFromTreeFocus(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	got := fakeOpener(a)
	a.focusTree = true
	drive(a, key("o"))
	if len(*got) != 1 {
		t.Fatalf("opened %v", *got)
	}
}

func TestOpenRejectsForeignURL(t *testing.T) {
	for _, u := range []string{"https://evil.example/x", "javascript:alert(1)", "http://vedabase.cc/x",
		"https://vedabase.cc.evil.example/", "https://user@evil.example/", "", "%zz"} {
		a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
		got := fakeOpener(a)
		a.item.URL = u
		drive(a, key("o"))
		if len(*got) != 0 || toastText(a) != "No web page for this item" {
			t.Errorf("%q: opened %v, toast %q", u, *got, toastText(a))
		}
	}
}

func TestOpenNothingOpen(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{})
	got := fakeOpener(a)
	a.hasItem = false
	drive(a, key("o"))
	if len(*got) != 0 || toastText(a) != "No web page for this item" {
		t.Fatalf("opened %v, toast %q", *got, toastText(a))
	}
}

func TestOpenFailureToasts(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.opener = func(string) error { return errors.New("no xdg-open") }
	drive(a, key("o"))
	if toastText(a) != "Couldn't open the browser" {
		t.Fatalf("toast %q", toastText(a))
	}
}

func TestWebButtonDrawnAndClickable(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	got := fakeOpener(a)
	row := strings.Split(plain(a), "\n")[1]
	if !strings.Contains(row, "↗ vedabase.cc") || !strings.HasSuffix(row, " ─╮") {
		t.Fatalf("top border %q", row)
	}
	r := a.webButtonRect()
	drive(a, click(r.x, r.y))
	if len(*got) != 1 {
		t.Fatalf("click opened %v", *got)
	}
	// one cell left of the button is not it
	*got = nil
	drive(a, click(r.x-1, r.y))
	if len(*got) != 0 {
		t.Fatalf("border click opened %v", *got)
	}
	// the button is where it is drawn
	cells := []rune(row)
	if string(cells[r.x:r.x+r.w]) != "↗ vedabase.cc" {
		t.Fatalf("drawn %q at rect", string(cells[r.x:r.x+r.w]))
	}
}

func TestWebButtonAbsentWithoutItem(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{})
	a.hasItem = false
	if strings.Contains(plain(a), "↗") {
		t.Fatal("button drawn with no item")
	}
}

func TestWebButtonDroppedWhenNarrow(t *testing.T) {
	if _, ok := titleRoom(30, 12); ok {
		t.Fatal("30 columns should drop the button")
	}
	if n, ok := titleRoom(60, 30); !ok || n < 12 {
		t.Fatalf("60 columns: room %d ok %v", n, ok)
	}
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	out := a.box([]string{"x"}, 30, 4, "Bhagavad-gītā 2.13", true, "", "")
	if strings.Contains(strings.Join(out, ""), "↗") {
		t.Fatal("button drawn without being asked")
	}
	// unframed: no button
	drive(a, tea_size(40, 8))
	if strings.Contains(plain(a), "↗") {
		t.Fatal("button in unframed mode")
	}
}

func TestSidebarTag(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	got := fakeOpener(a)
	tr, _ := a.paneRects()
	lines := strings.Split(plain(a), "\n")
	last := []rune(lines[tr.y+tr.h-1])[:tr.w]
	if !strings.Contains(string(last), " vedabase.cc ") || last[0] != '╰' {
		t.Fatalf("tree bottom border %q", string(last))
	}
	tag, ok := a.siteTagRect()
	if !ok || string([]rune(lines[tag.y])[tag.x:tag.x+tag.w]) != " vedabase.cc " {
		t.Fatalf("tag rect %+v ok=%v", tag, ok)
	}
	drive(a, click(tag.x+3, tag.y))
	if !reflect.DeepEqual(*got, []string{"https://vedabase.cc/"}) {
		t.Fatalf("opened %v", *got)
	}
	// hide the tree: no tag
	*got = nil
	drive(a, key("t"))
	if strings.Contains(plain(a), "─ vedabase.cc ─") {
		t.Fatal("tag drawn with the tree hidden")
	}
	if _, ok := a.siteTagRect(); ok {
		t.Fatal("tag rect with the tree hidden")
	}
}

func TestBrowserCommand(t *testing.T) {
	const u = "https://vedabase.cc/"
	for _, c := range []struct {
		goos string
		wsl  bool
		prog string
		args []string
	}{
		{"linux", true, "explorer.exe", []string{u}},
		{"linux", false, "xdg-open", []string{u}},
		{"darwin", false, "open", []string{u}},
		{"windows", false, "rundll32", []string{"url.dll,FileProtocolHandler", u}},
	} {
		p, args := browserCommand(c.goos, c.wsl, u)
		if p != c.prog || !reflect.DeepEqual(args, c.args) {
			t.Errorf("%s wsl=%v: %s %v", c.goos, c.wsl, p, args)
		}
	}
}

func tea_size(w, h int) tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} }
