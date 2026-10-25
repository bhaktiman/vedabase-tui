package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/render"
	"vedabase-tui/internal/store"
)

func typeKeys(a *App, s string) {
	for _, r := range s {
		drive(a, key(string(r)))
	}
}

// searchApp boots on bg/2/13 and runs a search for "soul".
func searchApp(t *testing.T) (*App, *fakeBackend) {
	t.Helper()
	be := newFake(t)
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("/"))
	typeKeys(a, "soul")
	return a, be
}

func TestSearchRunsOnEnterOnly(t *testing.T) {
	a, be := searchApp(t)
	if n := len(be.searches()); n != 0 {
		t.Fatalf("typing searched %d times", n)
	}
	drive(a, key("enter"))
	got := be.searches()
	if len(got) != 1 || got[0] != (searchCall{"soul", "", 1}) {
		t.Fatalf("calls = %+v", got)
	}
}

func TestSearchScope(t *testing.T) {
	a, be := searchApp(t)
	drive(a, key("tab"))
	if v := plain(a); !strings.Contains(v, "This book (Bhagavad-gītā As It Is)") {
		t.Fatalf("scope line missing book:\n%s", v)
	}
	drive(a, key("enter"))
	if got := be.searches(); got[0].book != "bg" {
		t.Fatalf("book = %q", got[0].book)
	}
	drive(a, key("/"), key("tab"), key("tab"), key("enter"))
	if got := be.searches(); len(got) != 2 || got[1].book != "transcripts" {
		// "/" reopens with the query kept; this->letters->lectures
		t.Fatalf("calls = %+v", got)
	}
	drive(a, key("/"), key("tab"), key("tab"), key("enter")) // lectures -> all -> this book
	drive(a, key("/"), key("tab"), key("enter"))
	if got := be.searches(); got[len(got)-1].book != "letters" {
		t.Fatalf("calls = %+v", got)
	}
}

func TestSearchPaging(t *testing.T) {
	a, be := searchApp(t)
	drive(a, key("enter"))
	if n := len(a.search.results); n != 2 {
		t.Fatalf("results = %d", n)
	}
	drive(a, key("j"))
	got := be.searches()
	if len(got) != 2 || got[1].page != 2 {
		t.Fatalf("calls = %+v", got)
	}
	if n := len(a.search.results); n != 3 {
		t.Fatalf("results after page 2 = %d", n)
	}
	drive(a, key("j"))
	if a.search.cursor != 2 || len(be.searches()) != 2 {
		t.Fatalf("cursor %d, calls %d", a.search.cursor, len(be.searches()))
	}
}

func TestSearchLoadingMoreRow(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("enter"))
	a.search.loading = true
	if v := plain(a); !strings.Contains(v, "Loading more…") {
		t.Fatalf("no loading row:\n%s", v)
	}
}

func TestSearchOpenAndNext(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("enter"))
	drive(a, key("enter")) // run, then open result 1
	if a.ref != "bg/2/13" || a.search.visible {
		t.Fatalf("ref %q visible %v", a.ref, a.search.visible)
	}
	drive(a, key("n"))
	if a.ref != "bg/2/14" {
		t.Fatalf("n: ref %q", a.ref)
	}
	drive(a, key("n")) // past the loaded page: loads page 2
	if a.ref != "bg/2/15" {
		t.Fatalf("n across pages: ref %q", a.ref)
	}
	drive(a, key("N"))
	drive(a, key("N"))
	if a.ref != "bg/2/13" {
		t.Fatalf("N: ref %q", a.ref)
	}
	drive(a, key("esc"))
	if !a.search.visible || a.search.cursor != 0 {
		t.Fatalf("esc: visible %v cursor %d", a.search.visible, a.search.cursor)
	}
	if v := plain(a); !strings.Contains(v, "bg/2/14") && !strings.Contains(v, "Bg 2.14") {
		t.Fatalf("results not shown:\n%s", v)
	}
	drive(a, key("esc"))
	if a.search.visible {
		t.Fatal("esc did not close")
	}
}

func TestSearchNextAtEnds(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("enter"))
	drive(a, key("enter"))
	drive(a, key("N"))
	if a.toasts.cur == nil || !strings.Contains(a.toasts.cur.text, "First") {
		t.Fatalf("toast = %+v", a.toasts.cur)
	}
}

func TestSearchMarksHighlighted(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("enter"))
	v := plain(a)
	if !strings.Contains(v, "soul") || strings.Contains(v, "<mark>") {
		t.Fatalf("view:\n%s", v)
	}
	if !strings.Contains(a.View().Content, "\x1b[") { // styled
		t.Fatal("no styling")
	}
}

func TestSearchRateLimited(t *testing.T) {
	a, be := searchApp(t)
	be.searchErr = &api.RateLimitedError{RetryAfter: 12 * time.Second}
	drive(a, key("enter"))
	if v := plain(a); !strings.Contains(v, "Wait 12s before searching again") {
		t.Fatalf("view:\n%s", v)
	}
	if a.toasts.cur != nil {
		t.Fatal("rate limit should not toast")
	}
}

func TestSearchOtherErrorToasts(t *testing.T) {
	a, be := searchApp(t)
	be.searchErr = &api.APIError{Status: 500, Message: "boom"}
	drive(a, key("enter"))
	if a.toasts.cur == nil || a.toasts.cur.text != "boom" || !a.toasts.cur.err {
		t.Fatalf("toast = %+v", a.toasts.cur)
	}
}

func TestSearchOverlayKeysDoNotLeak(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("q"), key("t"), key("1")) // typed into the box
	if a.state.TreeHidden || !a.parts.Devanagari || !a.search.visible {
		t.Fatal("keys leaked to the app")
	}
	if got := a.search.input.Value(); got != "soulqt1" {
		t.Fatalf("input = %q", got)
	}
}

func TestSearchNWithoutSearch(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("n"))
	if a.toasts.cur == nil {
		t.Fatal("expected a toast")
	}
	_ = tea.Quit
}

func TestSearchScopeLetters(t *testing.T) {
	a, be := searchApp(t)
	drive(a, key("tab"), key("tab"), key("enter"))
	if got := be.searches(); len(got) != 1 || got[0].book != "letters" {
		t.Fatalf("calls = %+v", got)
	}
}

func TestSearchDropsBadRefs(t *testing.T) {
	s := NewSearch(render.Default)
	s.onResults(searchResultsMsg{req: 1, page: api.SearchPage{Results: []api.SearchResult{
		{Ref: "bg/2/13\x1b[2J"}, {Ref: "bg/2/14"}, {Ref: "../x"}, {Ref: "BG/2"},
	}}})
	if len(s.results) != 1 || s.results[0].Ref != "bg/2/14" {
		t.Fatalf("results = %+v", s.results)
	}
}
