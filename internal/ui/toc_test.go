package ui

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

func loadTOC(t *testing.T, name string) api.TOC {
	t.Helper()
	data, err := os.ReadFile("../api/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var toc api.TOC
	if err := json.Unmarshal(data, &toc); err != nil {
		t.Fatal(err)
	}
	return toc
}

// smallBG is a Gītā contents with chapter 1 (verse 19 included) and chapter 2.
func smallBG(withV19 bool) api.TOC {
	ch1 := []api.TOCNode{{Ref: "bg/1/1", Label: "1.1"}}
	if withV19 {
		ch1 = append(ch1, api.TOCNode{Ref: "bg/1/19", Label: "1.19"})
	}
	return api.TOC{Book: "bg", Lang: "en", Title: "Gītā", Children: []api.TOCNode{
		{Ref: "bg/1", Title: "Chapter 1: From the API", Children: ch1},
		{Ref: "bg/2", Title: "Chapter 2: Also from the API", Children: []api.TOCNode{{Ref: "bg/2/13", Label: "2.13"}}},
	}}
}

func bootWith(t *testing.T, be *fakeBackend, ref string) *App {
	t.Helper()
	a := New(Options{Backend: be, State: store.State{LastRef: ref, Parts: api.AllParts()}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	return a
}

func TestCachedTOCShownOnFirstFrame(t *testing.T) {
	be := newFake(t)
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)}
	be.tocFresh = true
	a := New(Options{Backend: be, State: store.State{Parts: api.AllParts()}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a.Update(libraryMsg{lib: be.lib}) // only the library arrives; no TOC fetch has run
	if got := be.tocCalled(); len(got) != 0 {
		t.Fatalf("network TOC calls before the first frame: %v", got)
	}
	if n := a.lib.Find("bg/2"); n == nil || n.Title != "Chapter 2: Also from the API" {
		t.Fatalf("bg/2 = %+v", n)
	}
}

func TestFreshCachedTOCIsNotFetched(t *testing.T) {
	be := newFake(t)
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)}
	be.tocFresh = true
	bootWith(t, be, "bg/2/13")
	if slices.Contains(be.tocCalled(), "bg") {
		t.Errorf("fresh TOC refetched: %v", be.tocCalled())
	}
}

func TestStaleCachedTOCIsRefreshed(t *testing.T) {
	be := newFake(t)
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)}
	fresh := smallBG(true)
	fresh.Children[1].Title = "Chapter 2: Refreshed"
	be.tocs = map[string]api.TOC{"bg": fresh}
	a := bootWith(t, be, "bg/2/13")
	if got := be.tocCalled(); got[0] != "bg" {
		t.Errorf("calls %v", got)
	}
	if n := a.lib.Find("bg/2"); n.Title != "Chapter 2: Refreshed" {
		t.Errorf("title %q", n.Title)
	}
}

func TestTOCQueueOrder(t *testing.T) {
	be := newFake(t)
	bootWith(t, be, "bg/2/13")
	if got, want := be.tocCalled(), []string{"bg", "sb", "cc", "iso", "lcfl", "zzz"}; !slices.Equal(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
	be = newFake(t)
	bootWith(t, be, "cc/madhya/9/74")
	if got, want := be.tocCalled(), []string{"cc", "bg", "sb", "iso", "lcfl", "zzz"}; !slices.Equal(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}

func TestExpandingABookMovesItToTheFront(t *testing.T) {
	be := newFake(t)
	var a *App
	once := false
	be.onTOC = func(code string) {
		if once {
			return
		}
		once = true // the first fetch (bg) is under way: the reader expands lcfl
		a.focusTree = true
		a.tree.Reveal(a.lib.Find("lcfl"))
		drive(a, key("l"))
	}
	a = New(Options{Backend: be, State: store.State{LastRef: "bg/2/13", Parts: api.AllParts()}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	if got, want := be.tocCalled(), []string{"bg", "lcfl", "sb", "cc", "iso", "zzz"}; !slices.Equal(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}

func TestExpandedBooksComeBeforeTheRest(t *testing.T) {
	be := newFake(t)
	be.tocFresh = true
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)} // not fetched, so nothing runs before the expand
	var a *App
	be.onTOC = func(code string) {
		if code != "sb" {
			return
		}
		be.onTOC = nil
		a.focusTree = true
		a.tree.Reveal(a.lib.Find("iso"))
		drive(a, key("l"))
	}
	a = New(Options{Backend: be, State: store.State{LastRef: "bg/2/13", Parts: api.AllParts()}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	if got, want := be.tocCalled(), []string{"sb", "iso", "cc", "lcfl", "zzz"}; !slices.Equal(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}

func TestTOCKeepsCursorAndNode(t *testing.T) {
	be := newFake(t)
	a := bootWith(t, be, "bg/2/13")
	a.focusTree = true
	a.tree.Reveal(a.lib.Find("bg/1/19"))
	oldNode := a.node
	drive(a, tocMsg{code: "bg", lang: "en", toc: smallBG(true)})
	if n := a.tree.Selected(); n == nil || n.Ref != "bg/1/19" || n != a.lib.Find("bg/1/19") {
		t.Errorf("cursor on %+v", n)
	}
	if a.node == nil || a.node.Ref != "bg/2/13" || a.node == oldNode || a.node != a.lib.Find("bg/2/13") {
		t.Errorf("current node %+v (old %p)", a.node, oldNode)
	}
	if !a.tree.expanded["bg/2"] {
		t.Error("expansion lost")
	}
	if !strings.Contains(plain(a), "Chapter 2: Also from the API") {
		t.Errorf("new title not shown:\n%s", plain(a))
	}
}

func TestTOCMovesCursorToBookRootWhenRefVanishes(t *testing.T) {
	be := newFake(t)
	a := bootWith(t, be, "bg/2/13")
	a.focusTree = true
	a.tree.Reveal(a.lib.Find("bg/1/19"))
	drive(a, tocMsg{code: "bg", lang: "en", toc: smallBG(false)})
	if n := a.tree.Selected(); n == nil || n.Ref != "bg" {
		t.Errorf("cursor on %+v, want the book root", n)
	}
}

func TestTOCFilterResultsSurvive(t *testing.T) {
	be := newFake(t)
	a := bootWith(t, be, "bg/2/13")
	a.tree.SetFilter("2.13")
	drive(a, tocMsg{code: "bg", lang: "en", toc: smallBG(true)})
	for n := range a.tree.shown {
		if a.lib.Find(n.Ref) != n && n.Parent != nil && n.Book == "bg" {
			t.Errorf("filter holds a stale node %s", n.Ref)
		}
	}
}

func TestTOCOfAnotherLanguageIsDropped(t *testing.T) {
	be := newFake(t)
	a := bootWith(t, be, "bg/2/13")
	drive(a, tocMsg{code: "bg", lang: "ru", toc: smallBG(true)})
	if n := a.lib.Find("bg/2"); n.Title == "Chapter 2: Also from the API" {
		t.Error("foreign-language TOC applied")
	}
	if _, ok := a.tocs["bg"]; ok {
		t.Error("foreign-language TOC kept")
	}
}

func TestTOCSurvivesLibraryRefresh(t *testing.T) {
	be := newFake(t)
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)}
	be.tocFresh = true
	a := bootWith(t, be, "bg/2/13")
	drive(a, libraryMsg{lib: sampleLibrary(t)})
	if n := a.lib.Find("bg/2"); n == nil || n.Title != "Chapter 2: Also from the API" {
		t.Errorf("bg/2 = %+v", n)
	}
	if a.node != a.lib.Find("bg/2/13") {
		t.Error("current node not re-found")
	}
}

func TestNoScrapingForBooksWithATOC(t *testing.T) {
	be := newFake(t)
	be.scrapeFixtures()
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)}
	be.tocFresh = true
	a := bootWith(t, be, "bg/2/13")
	a.focusTree = true
	a.tree.Reveal(a.lib.Find("bg"))
	drive(a, key("h"), key("l"))
	if got := be.pagesCalled(); len(got) != 0 {
		t.Errorf("scraped %v", got)
	}
}

func TestFailedTOCDoesNotStopTheQueue(t *testing.T) {
	be := newFake(t)
	be.tocs = map[string]api.TOC{"cc": loadTOC(t, "toc-cc-trim.json")}
	bootWith(t, be, "bg/2/13")
	if got := be.tocCalled(); len(got) != 6 {
		t.Errorf("calls %v", got)
	}
}
