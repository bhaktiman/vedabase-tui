package ui

import (
	"errors"
	"slices"
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

func TestChapterTitlesApplied(t *testing.T) {
	be := newFake(t)
	be.scrapeFixtures()
	a := treeAt(t, be, "bg")
	drive(a, key("h")) // collapse bg
	drive(a, key("l"))
	if !strings.Contains(plain(a), "Chapter 2: Contents of the Gītā Summarized") {
		t.Errorf("title missing:\n%s", plain(a))
	}
	if n := a.lib.Find("bg/2"); n == nil || n.Title != "Chapter 2: Contents of the Gītā Summarized" {
		t.Errorf("node %+v", n)
	}
	// once per session
	drive(a, key("h"), key("l"))
	if got := slices.Compact(be.pagesCalled()); len(got) != 1 {
		t.Errorf("page calls %v", be.pagesCalled())
	}
}

func TestLettersByYearLazy(t *testing.T) {
	be := newFake(t)
	be.scrapeFixtures()
	a := treeAt(t, be, "letters#year")
	drive(a, key("l"))
	if !strings.Contains(plain(a), "1967 (264)") {
		t.Fatalf("year row missing:\n%s", plain(a))
	}
	if n := a.lib.Find("letters#year"); n.Label() != "By year" {
		t.Errorf("By year label %q", n.Label())
	}
	drive(a, key("j"))
	for a.tree.Selected().Seg != "1967" {
		drive(a, key("j"))
	}
	drive(a, key("l"))
	g := a.lib.Find("letters#year/1967")
	if len(g.Children) != 20 {
		t.Fatalf("%d letters", len(g.Children))
	}
	if g.Children[0].Label() != `Letter to: "Children" at New York` {
		t.Errorf("first label %q", g.Children[0].Label())
	}
	if n := a.lib.Find(g.Children[1].Ref); n.Label() != "Letter to: Aatie" {
		t.Errorf("by-recipient label %q", n.Label())
	}
	before := len(be.pagesCalled())
	for range 19 {
		drive(a, key("j"))
	}
	if len(be.pagesCalled()) != before {
		t.Fatal("next page fetched too early")
	}
	drive(a, key("j"))
	calls := be.pagesCalled()
	if len(calls) != before+1 || calls[len(calls)-1] != "https://vedabase.cc/en/library/letters/?year=1967&page=2" {
		t.Errorf("calls %v", calls)
	}
}

func TestTitleFailureKeepsDefaults(t *testing.T) {
	be := newFake(t)
	be.scrapeFixtures()
	be.pageErr = errors.New("boom")
	a := treeAt(t, be, "bg")
	drive(a, key("h"), key("l"))
	if !strings.Contains(plain(a), "Chapter 2") || strings.Contains(plain(a), "Contents of") {
		t.Errorf("default labels expected:\n%s", plain(a))
	}
	if a.toasts.cur != nil {
		t.Error("failure toasted")
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
