package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

// manyLinksApp shows an item whose translation cites Bg. 4.1 … Bg. 4.n, one
// per paragraph.
func manyLinksApp(t *testing.T, n int) *App {
	t.Helper()
	be := newFake(t)
	data, err := os.ReadFile("../api/testdata/verse-bg-2-13.json")
	if err != nil {
		t.Fatal(err)
	}
	var it api.Item
	if err := json.Unmarshal(data, &it); err != nil {
		t.Fatal(err)
	}
	var tr []string
	it.References = nil
	for i := 1; i <= n; i++ {
		s := fmt.Sprintf("Bg. 4.%d", i)
		tr = append(tr, "See "+s+" here.")
		it.References = append(it.References, api.Reference{Text: s, Ref: fmt.Sprintf("bg/4/%d", i), Field: "translation"})
	}
	it.Translation = strings.Join(tr, "\n\n")
	it.Purport = ""
	be.items["bg/2/13"] = it
	st := store.State{Parts: api.AllParts(), LastRef: "bg/2/13", TreeHidden: true}
	a := New(Options{Backend: be, State: st, Settings: store.Settings{Mouse: true, ReadingWidth: 60}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 100, Height: 80}, a.Init()())
	return a
}

func TestHintLabelsAssignment(t *testing.T) {
	if got := hintLabels(3); strings.Join(got, ",") != "a,s,d" {
		t.Fatalf("3 = %v", got)
	}
	if got := hintLabels(16); got[15] != "w" || len(got[0]) != 1 {
		t.Fatalf("16 = %v", got)
	}
	got := hintLabels(20)
	if got[0] != "aa" || got[1] != "as" || got[16] != "sa" || len(got[19]) != 2 {
		t.Fatalf("20 = %v", got)
	}
}

func TestHintsOnlyVisibleLinksInOrder(t *testing.T) {
	a := manyLinksApp(t, 40)
	vis := a.reader.VisibleLinks()
	if len(vis) == 0 || len(vis) >= 40 {
		t.Fatalf("visible = %d", len(vis))
	}
	drive(a, key("f"))
	if !a.hints.active || len(a.hints.items) != len(vis) {
		t.Fatalf("hints = %+v", a.hints)
	}
	for i, h := range a.hints.items {
		if h.link != vis[i] {
			t.Fatalf("item %d = %+v, want %+v", i, h.link, vis[i])
		}
	}
	if !strings.Contains(plain(a), "a") || !strings.Contains(plain(a), "type a label") {
		t.Fatal("no bottom bar text")
	}
}

func TestHintsTwoLettersFor20Links(t *testing.T) {
	a := manyLinksApp(t, 20)
	drive(a, key("f"))
	if len(a.hints.items) != 20 {
		t.Fatalf("items = %d", len(a.hints.items))
	}
	for _, h := range a.hints.items {
		if len(h.label) != 2 {
			t.Fatalf("label %q", h.label)
		}
	}
	// Narrowing: after "a" only labels starting with a remain, drawn as suffix.
	drive(a, key("a"))
	if !a.hints.active || a.hints.typed != "a" {
		t.Fatalf("typed = %q", a.hints.typed)
	}
	// "a" + "s" is the second link.
	drive(a, key("s"))
	if a.hints.active || a.ref != "bg/4/2" || len(a.back) != 1 || a.back[0].ref != "bg/2/13" {
		t.Fatalf("ref = %q back = %+v active=%v", a.ref, a.back, a.hints.active)
	}
}

func TestHintsSingleLetterOpens(t *testing.T) {
	a := manyLinksApp(t, 3)
	drive(a, key("f"), key("d"))
	if a.ref != "bg/4/3" || len(a.back) != 1 || a.hints.active {
		t.Fatalf("ref = %q back = %+v", a.ref, a.back)
	}
}

func TestHintsCancel(t *testing.T) {
	for _, k := range []string{"esc", "z", "x", "j"} {
		a := manyLinksApp(t, 3)
		drive(a, key("f"), key(k))
		if a.hints.active || a.ref != "bg/2/13" || a.reader.Offset() != 0 {
			t.Fatalf("%s: active=%v ref=%q", k, a.hints.active, a.ref)
		}
	}
	a := manyLinksApp(t, 20)
	drive(a, key("f"), key("d"), key("a")) // none of the 20 labels starts with "da"
	if a.hints.active || a.ref != "bg/2/13" {
		t.Fatal("bad prefix did not cancel")
	}
	a = manyLinksApp(t, 3)
	drive(a, key("f"), click(0, 0))
	if a.hints.active {
		t.Fatal("click did not cancel")
	}
	a = manyLinksApp(t, 3)
	drive(a, key("f"), tea.WindowSizeMsg{Width: 90, Height: 70})
	if a.hints.active {
		t.Fatal("resize did not cancel")
	}
}

func TestHintsNoLinks(t *testing.T) {
	a := manyLinksApp(t, 0)
	drive(a, key("f"))
	if a.hints.active || a.toasts.cur == nil || a.toasts.cur.text != "No links on screen" {
		t.Fatalf("active=%v toast=%+v", a.hints.active, a.toasts.cur)
	}
}

func TestHintsDrawnOverLinkStart(t *testing.T) {
	a := manyLinksApp(t, 3)
	x, y := cellOf(t, a, "Bg. 4.1")
	drive(a, key("f"))
	lines := strings.Split(plain(a), "\n")
	if got := string([]rune(lines[y])[x : x+1]); got != "a" {
		t.Fatalf("cell = %q in %q", got, lines[y])
	}
	drive(a, key("s"))
}

func TestTreeFocusFStillFilters(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("tab"))
	if !a.focusTree {
		t.Skip("tree not focused")
	}
	drive(a, key("f"))
	if !a.filtering || a.hints.active {
		t.Fatalf("filtering=%v hints=%v", a.filtering, a.hints.active)
	}
}
