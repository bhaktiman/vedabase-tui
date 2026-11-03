package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"

	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/store"
)

func betaRefs(cs *collections.Store) string {
	c, _ := cs.Get("Beta")
	var out []string
	for _, it := range c.Items {
		out = append(out, it.Ref)
	}
	return strings.Join(out, ",")
}

func inBetaRef(r string) func(*library.Node) bool {
	return func(n *library.Node) bool { return n.Ref == r && n.Parent != nil && n.Parent.Seg == "Beta" }
}

func TestSidebarMoveItem(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, inBetaRef("bg/2/1"))
	drive(a, key("K"))
	if got := betaRefs(cs); got != "bg/2/1,bg/2/13" {
		t.Fatalf("after K: %s", got)
	}
	if !a.focusTree {
		t.Fatal("focus left the sidebar")
	}
	if n := a.tree.Selected(); n == nil || !inBetaRef("bg/2/1")(n) {
		t.Fatalf("cursor on %+v, want the moved item", n)
	}
	kids := a.collectionNode("Beta").Children
	if kids[0].Ref != "bg/2/1" {
		t.Fatalf("sidebar order: %s first", kids[0].Ref)
	}
	drive(a, key("K")) // at the top: clamped, nothing changes
	if got := betaRefs(cs); got != "bg/2/1,bg/2/13" {
		t.Fatalf("clamped K: %s", got)
	}
	drive(a, key("J"))
	if got := betaRefs(cs); got != "bg/2/13,bg/2/1" {
		t.Fatalf("after J: %s", got)
	}
	if n := a.tree.Selected(); n == nil || !inBetaRef("bg/2/1")(n) {
		t.Fatalf("cursor on %+v after J", n)
	}
}

func TestMoveKeysIgnoredOffCollectionItems(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, func(n *library.Node) bool {
		return n.Seg == "Beta" && n.Parent != nil && n.Parent.Ref == library.CollectionsRef
	})
	drive(a, key("J"), key("K"))
	if got := betaRefs(cs); got != "bg/2/13,bg/2/1" {
		t.Fatalf("collection row moved items: %s", got)
	}
	selectNode(t, a, inBetaRef("bg/2/1"))
	a.focusTree = false
	drive(a, key("K"))
	if got := betaRefs(cs); got != "bg/2/13,bg/2/1" {
		t.Fatalf("unfocused sidebar moved items: %s", got)
	}
}

func TestOverlayMoveItem(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	a.colview.ShowIn(cs, "Beta")
	if v := plain(a); !strings.Contains(v, "J/K move") {
		t.Fatalf("hint lacks J/K move:\n%s", v)
	}
	drive(a, key("j"), key("K"))
	if got := betaRefs(cs); got != "bg/2/1,bg/2/13" {
		t.Fatalf("after K: %s", got)
	}
	if it, ok := a.colview.selectedItem(); !ok || it.Ref != "bg/2/1" {
		t.Fatalf("cursor on %+v, want bg/2/1", it)
	}
	drive(a, key("J"))
	if got := betaRefs(cs); got != "bg/2/13,bg/2/1" {
		t.Fatalf("after J: %s", got)
	}
	if it, ok := a.colview.selectedItem(); !ok || it.Ref != "bg/2/1" {
		t.Fatalf("cursor on %+v after J", it)
	}
}

func TestStepFollowsMovedOrder(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, inBetaRef("bg/2/1"))
	drive(a, key("K")) // Beta: bg/2/1, bg/2/13
	if got := betaRefs(cs); got != "bg/2/1,bg/2/13" {
		t.Fatalf("order %s", got)
	}
	drive(a, key("enter")) // open bg/2/1 inside Beta
	if a.ref != "bg/2/1" {
		t.Fatalf("opened %q", a.ref)
	}
	drive(a, key("l"))
	if a.ref != "bg/2/13" {
		t.Fatalf("l went to %q, want bg/2/13", a.ref)
	}
}

func TestCollectionsHintsMention(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, inBetaRef("bg/2/1"))
	h, ok := a.collectionsHints()
	if !ok || !strings.Contains(strings.Join(h, "|"), "J/K move") {
		t.Fatalf("hints %v", h)
	}
}

// twoWindowCols is jumpApp over a file a second Store also has open.
func twoWindowCols(t *testing.T) (*App, *collections.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "collections.json")
	cs, _, err := collections.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := collections.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{Backend: newFake(t), Collections: cs, State: store.State{LastRef: "bg/2/13", Parts: api.AllParts()},
		Now: func() time.Time { return time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) }})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	for _, r := range []string{"bg/2/13", "bg/2/1"} {
		if _, err := cs.Add("Beta", collections.Item{Ref: r, Book: "bg", Label: "L " + r, Added: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatal(err)
		}
	}
	drive(a, key("l"))
	return a, other
}

func TestSidebarMoveItemRemovedElsewhere(t *testing.T) {
	a, other := twoWindowCols(t)
	selectNode(t, a, inBetaRef("bg/2/1"))
	if err := other.Remove("Beta", "bg/2/1"); err != nil {
		t.Fatal(err)
	}
	drive(a, key("K"))
	if got := toastText(a); got != "That item is no longer in Beta" {
		t.Fatalf("toast %q", got)
	}
	for _, c := range a.collectionNode("Beta").Children {
		if c.Ref == "bg/2/1" {
			t.Fatal("sidebar still lists the removed item")
		}
	}
}

func TestOverlayMoveItemRemovedElsewhere(t *testing.T) {
	a, other := twoWindowCols(t)
	openColView(a)
	drive(a, key("enter"), key("j"))
	if err := other.Remove("Beta", "bg/2/1"); err != nil {
		t.Fatal(err)
	}
	drive(a, key("K"))
	if got := toastText(a); got != "That item is no longer in Beta" || a.colview.cursor != 0 {
		t.Fatalf("toast %q cursor %d", got, a.colview.cursor)
	}
}
