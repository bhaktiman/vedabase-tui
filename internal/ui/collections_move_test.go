package ui

import (
	"strings"
	"testing"

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
