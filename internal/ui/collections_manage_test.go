package ui

import (
	"strings"
	"testing"

	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/store"
)

// selectNode puts the sidebar cursor on the node for which pick is true.
func selectNode(t *testing.T, a *App, pick func(*library.Node) bool) {
	t.Helper()
	for _, r := range a.lib.Roots {
		if r.Ref != library.CollectionsRef {
			continue
		}
		for _, c := range r.Children {
			a.tree.ExpandNode(c)
		}
		a.tree.Reveal(r.Children[0])
		for _, c := range r.Children {
			if pick(c) {
				a.tree.Reveal(c)
				a.focusTree = true
				return
			}
			for _, it := range c.Children {
				if pick(it) {
					a.tree.Reveal(it)
					a.focusTree = true
					return
				}
			}
		}
	}
	t.Fatal("node not found")
}

func seg(s string) func(*library.Node) bool {
	return func(n *library.Node) bool { return n.Seg == s }
}

func treeHas(a *App, s string) bool {
	for _, r := range a.tree.rows {
		if r.node.Seg == s || r.node.Title == s {
			return true
		}
	}
	return false
}

func TestSidebarDeleteCollection(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, seg("Alpha"))
	drive(a, key("D"))
	if !a.colview.visible || !strings.Contains(plain(a), "Delete Alpha? y/n") {
		t.Fatalf("no delete confirm:\n%s", plain(a))
	}
	drive(a, key("y"))
	if _, ok := cs.Get("Alpha"); ok {
		t.Fatal("still in store")
	}
	if treeHas(a, "Alpha") {
		t.Fatal("still in tree")
	}
}

func TestSidebarRenameCollection(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, seg("Beta"))
	drive(a, key("R"))
	if !strings.Contains(plain(a), "Rename to: Beta") {
		t.Fatalf("no prefilled rename:\n%s", plain(a))
	}
	typeKeys(a, "2")
	drive(a, key("enter"))
	if _, ok := cs.Get("Beta2"); !ok {
		t.Fatalf("store %+v", cs.List())
	}
	if !treeHas(a, "Beta2") || treeHas(a, "Beta") {
		t.Fatal("tree not renamed")
	}
}

func TestSidebarRemoveItem(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	inBeta := func(r string) func(*library.Node) bool {
		return func(n *library.Node) bool { return n.Ref == r && n.Parent != nil && n.Parent.Seg == "Beta" }
	}
	// Beta holds bg/2/13 then bg/2/1. Remove the first: no dialog, the
	// sidebar keeps the focus, and the cursor goes to the next item.
	selectNode(t, a, inBeta("bg/2/13"))
	drive(a, key("d"))
	if a.colview.visible {
		t.Fatal("remove opened the collections overlay")
	}
	col, _ := cs.Get("Beta")
	if len(col.Items) != 1 || col.Items[0].Ref != "bg/2/1" {
		t.Fatalf("items %+v", col.Items)
	}
	if v := plain(a); !strings.Contains(v, "Removed L bg/2/13 from Beta") {
		t.Fatalf("no toast:\n%s", v)
	}
	if !a.focusTree {
		t.Fatal("focus left the sidebar")
	}
	if n := a.tree.Selected(); n == nil || !inBeta("bg/2/1")(n) {
		t.Fatalf("cursor on %+v, want Beta's bg/2/1", n)
	}
	for _, r := range a.tree.rows {
		if inBeta("bg/2/13")(r.node) {
			t.Fatal("item still in tree")
		}
	}
	// The last item: the cursor goes to the (now empty) collection row.
	drive(a, key("d"))
	if n := a.tree.Selected(); n == nil || n.Seg != "Beta" || n.Parent == nil || n.Parent.Ref != library.CollectionsRef {
		t.Fatalf("cursor on %+v, want the Beta row", n)
	}
	if col, _ := cs.Get("Beta"); len(col.Items) != 0 {
		t.Fatalf("items %+v", col.Items)
	}
}

func TestSidebarDeleteCancel(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, seg("Alpha"))
	drive(a, key("D"), key("n"))
	if _, ok := cs.Get("Alpha"); !ok {
		t.Fatal("deleted on n")
	}
}

func TestDAndDoNothingElsewhere(t *testing.T) {
	a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	selectNode(t, a, seg("Alpha"))
	a.focusTree = false
	drive(a, key("D"), key("d"))
	if a.colview.visible {
		t.Fatal("opened with reader focus")
	}
	a.focusTree = true
	for _, r := range a.lib.Roots {
		if r.Ref == library.CollectionsRef {
			a.tree.Reveal(r)
		}
	}
	drive(a, key("D"), key("d"))
	if a.colview.visible || len(cs.List()) != 2 {
		t.Fatal("opened on the root")
	}
}

func TestRStillRefetchesElsewhere(t *testing.T) {
	cases := map[string]func(*testing.T, *App){
		"reader": func(t *testing.T, a *App) { a.focusTree = false },
		"item": func(t *testing.T, a *App) {
			selectNode(t, a, func(n *library.Node) bool { return n.Ref == "bg/2/1" && n.Parent.Seg == "Beta" })
		},
		"book": func(t *testing.T, a *App) {
			a.focusTree = true
			for _, r := range a.lib.Roots {
				if r.Ref != library.CollectionsRef && len(r.Children) > 0 {
					a.tree.Reveal(r)
					return
				}
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			a, _, _ := colApp(t, f, store.State{LastRef: "bg/2/13"})
			jumpFill(t, a)
			setup(t, a)
			_, cmd := a.Update(key("R"))
			if a.colview.visible {
				t.Fatal("rename opened")
			}
			if cmd == nil {
				t.Fatal("no refresh cmd")
			}
			drive(a, cmd())
			if len(f.refreshCalls) != 1 {
				t.Fatalf("refresh calls %v", f.refreshCalls)
			}
		})
	}
}

func TestBottomBarCollectionsHints(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	line := func() string { return ansiRe.ReplaceAllString(bottomLine(a), "") }
	selectNode(t, a, seg("Alpha"))
	if l := line(); !strings.Contains(l, "R rename · D delete · e export · c jump · ? help") {
		t.Fatalf("collection row: %q", l)
	}
	selectNode(t, a, func(n *library.Node) bool { return n.Ref == "bg/1/1" })
	if l := line(); !strings.Contains(l, "d remove · J/K move · c jump · ? help") || strings.Contains(l, "R rename") {
		t.Fatalf("item row: %q", l)
	}
	for _, r := range a.lib.Roots {
		if r.Ref == library.CollectionsRef {
			a.tree.Reveal(r)
		}
	}
	if l := line(); !strings.Contains(l, "c jump · s add · ? help") {
		t.Fatalf("root: %q", l)
	}
	a.focusTree = false
	if l := line(); strings.Contains(l, "c jump") || !strings.Contains(l, "/ search") {
		t.Fatalf("reader focus: %q", l)
	}
	a.focusTree = true
	selectNode(t, a, seg("Alpha"))
	a.w = 95
	if l := line(); !strings.Contains(l, "? help") {
		t.Fatalf("narrow: %q", l)
	}
}

// jumpFill adds the jumpApp collections to an app already built.
func jumpFill(t *testing.T, a *App) {
	t.Helper()
	b, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
	_ = b
	for _, c := range cs.List() {
		for _, it := range c.Items {
			if _, err := a.cols.Add(c.Name, it); err != nil {
				t.Fatal(err)
			}
		}
	}
	drive(a, key("l"))
}

// The remove prompt names the item as the list shows it: a letter by its
// title, a verse by its reference, not by a raw ref like "letters letter-to-x".
func TestRemovePromptUsesLabel(t *testing.T) {
	for _, c := range []struct{ ref, label, want string }{
		{"letters/letter-to-aditi", "Letter to: Aditi", "Remove Letter to: Aditi? y/n"},
		{"bg/2/2", "Bg 2.2 — My dear Arjuna", "Remove Bg 2.2? y/n"},
		{"bg/2/3", "", "Remove Bg 2.3? y/n"},
	} {
		a, cs := jumpApp(t, store.State{LastRef: "bg/2/13"})
		if _, err := cs.Add("Gamma", collections.Item{Ref: c.ref, Book: "bg", Label: c.label}); err != nil {
			t.Fatal(err)
		}
		a.colview.ShowIn(cs, "Gamma")
		drive(a, key("d"))
		if v := plain(a); !strings.Contains(v, c.want) {
			t.Errorf("label %q: prompt lacks %q:\n%s", c.label, c.want, v)
		}
	}
}
