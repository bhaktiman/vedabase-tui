package ui

import (
	"errors"
	"strings"
	"testing"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

// treeAt starts an app with the tree focused and the cursor on ref.
func treeAt(t *testing.T, be *fakeBackend, ref string) *App {
	t.Helper()
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("tab"))
	a.tree.Reveal(a.lib.Find(ref))
	return a
}

// openYear expands By year and 1967 and returns the app with the cursor on 1967.
func openYear(t *testing.T, be *fakeBackend) *App {
	t.Helper()
	be.lettersFixtures()
	a := treeAt(t, be, "letters#year")
	drive(a, key("l"))
	for a.tree.Selected().Seg != "1967" {
		drive(a, key("j"))
	}
	drive(a, key("l"))
	return a
}

func TestLettersByYearFromTheAPI(t *testing.T) {
	be := newFake(t)
	a := openYear(t, be)
	if !strings.Contains(plain(a), "1967 (25)") || !strings.Contains(plain(a), "1966 (1)") {
		t.Fatalf("year rows missing:\n%s", plain(a))
	}
	if n := a.lib.Find("letters#year"); n.Label() != "By year" {
		t.Errorf("By year label %q", n.Label())
	}
	g := a.lib.Find("letters#year/1967")
	if len(g.Children) != 20 {
		t.Fatalf("%d letters", len(g.Children))
	}
	if g.Children[1].Label() != "Letter to: Abhirama" {
		t.Errorf("first label %q", g.Children[1].Label())
	}
	if n := a.lib.Find(g.Children[1].Ref); n.Label() != "Letter to: Abhirama" || n == g.Children[1] {
		t.Errorf("by-recipient node %+v", n)
	}
	drive(a, key("j")) // onto the first letter
	before := len(be.lettersCalled())
	for range 18 {
		drive(a, key("j"))
	}
	if len(be.lettersCalled()) != before {
		t.Fatalf("next page fetched too early: %v", be.lettersCalled())
	}
	drive(a, key("j")) // the 20th letter
	if calls := be.lettersCalled(); len(calls) != before+1 || calls[len(calls)-1] != "1967/2" {
		t.Fatalf("calls %v", calls)
	}
	if len(g.Children) != 25 {
		t.Errorf("%d letters after page 2", len(g.Children))
	}
	for range 10 { // has_more was false: no page 3
		drive(a, key("j"))
	}
	if calls := be.lettersCalled(); calls[len(calls)-1] != "1967/2" {
		t.Errorf("calls %v", calls)
	}
}

func TestYearsFailureRetriesOnNextExpand(t *testing.T) {
	be := newFake(t)
	be.lettersFixtures()
	be.yearsErr = errors.New("boom")
	a := treeAt(t, be, "letters#year")
	drive(a, key("l"))
	if a.toasts.cur != nil {
		t.Error("failure toasted")
	}
	be.yearsErr = nil
	drive(a, key("h"), key("l"))
	if !strings.Contains(plain(a), "1967 (25)") {
		t.Errorf("years missing after retry:\n%s", plain(a))
	}
}

func TestOpeningAByYearLetterKeepsTheCursorUnderByYear(t *testing.T) {
	be := newFake(t)
	a := openYear(t, be)
	drive(a, key("j"), key("j")) // the second letter
	want := a.tree.Selected()
	if want.Parent.Ref != "letters#year/1967" {
		t.Fatalf("cursor on %q", want.Ref)
	}
	drive(a, key("enter"))
	if a.ref != want.Ref {
		t.Fatalf("ref %q", a.ref)
	}
	if got := a.tree.Selected(); got != want {
		t.Errorf("cursor moved to %q under %q", got.Ref, got.Parent.Ref)
	}
	if a.node != want {
		t.Error("current node is not the By-year row")
	}
}

func TestStepFromAByYearLetterFollowsTheSiteOrderWithinTheYear(t *testing.T) {
	be := newFake(t)
	a := openYear(t, be)
	drive(a, key("j"), key("enter")) // first letter
	be.items["letters/letter-to-abhirama"] = api.Item{Ref: "letters/letter-to-abhirama", Book: "letters", Translation: "x"}
	a.item.Next = "letters/letter-to-abhirama"
	drive(a, key("l"))
	if a.ref != "letters/letter-to-abhirama" {
		t.Fatalf("ref %q", a.ref)
	}
	if sel := a.tree.Selected(); sel.Parent == nil || sel.Parent.Ref != "letters#year/1967" || sel.Ref != a.ref {
		t.Errorf("cursor on %q", sel.Ref)
	}
	// a target that is not loaded in By year falls back to the ref's own node
	if n := a.yearLetter(a.node, "letters/letter-to-nobody"); n != nil {
		t.Errorf("found %v", n)
	}
}

func TestChaptersKeepDefaultLabelsWithoutATOC(t *testing.T) {
	be := newFake(t)
	a := treeAt(t, be, "bg")
	drive(a, key("h"), key("l"))
	if !strings.Contains(plain(a), "Chapter 2") || strings.Contains(plain(a), "Contents of") {
		t.Errorf("default labels expected:\n%s", plain(a))
	}
}

func TestItemTitleFromTranslation(t *testing.T) {
	be := newFake(t)
	be.items["bg/0a"] = api.Item{Ref: "bg/0a", Book: "bg", Translation: "Preface"}
	a, _ := boot(t, be, store.State{LastRef: "bg/0a"})
	if got := a.lib.Find("bg/0a").Label(); got != "Preface" {
		t.Errorf("label %q", got)
	}
}
