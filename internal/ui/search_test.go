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

// reopened runs a search, opens result 1 and presses / again.
func reopened(t *testing.T) (*App, *fakeBackend) {
	t.Helper()
	a, be := searchApp(t)
	drive(a, key("enter"), key("enter"), key("/"))
	if !a.search.visible || !a.search.focusInput || a.search.cursor != 0 {
		t.Fatalf("reopen: visible %v focus %v cursor %d", a.search.visible, a.search.focusInput, a.search.cursor)
	}
	return a, be
}

func TestSearchReopenCtrlNIntoResults(t *testing.T) {
	a, _ := reopened(t)
	drive(a, key("ctrl+n"))
	if a.search.cursor != 1 || a.search.focusInput {
		t.Fatalf("ctrl+n: cursor %d focusInput %v", a.search.cursor, a.search.focusInput)
	}
	drive(a, key("j"))
	if a.search.cursor != 2 || a.search.input.Value() != "soul" {
		t.Fatalf("j: cursor %d query %q", a.search.cursor, a.search.input.Value())
	}
	drive(a, key("enter"))
	if a.ref != a.search.results[2].Ref || a.search.visible {
		t.Fatalf("ref %q visible %v", a.ref, a.search.visible)
	}
}

func TestSearchReopenArrowThenJ(t *testing.T) {
	a, _ := reopened(t)
	drive(a, key("down"))
	drive(a, key("j"))
	if a.search.cursor != 2 || a.search.input.Value() != "soul" || a.search.focusInput {
		t.Fatalf("cursor %d query %q focus %v", a.search.cursor, a.search.input.Value(), a.search.focusInput)
	}
	drive(a, key("/"), key("up"), key("up"), key("k"))
	if a.search.cursor != 0 || a.search.focusInput || a.search.input.Value() != "soul" {
		t.Fatalf("up: cursor %d focus %v query %q", a.search.cursor, a.search.focusInput, a.search.input.Value())
	}
}

func TestSearchCtrlPFromBoxStaysOnFirst(t *testing.T) {
	a, _ := reopened(t)
	drive(a, key("ctrl+p"))
	if a.search.cursor != 0 || a.search.focusInput {
		t.Fatalf("cursor %d focus %v", a.search.cursor, a.search.focusInput)
	}
	drive(a, key("ctrl+n"), key("ctrl+p"))
	if a.search.cursor != 0 {
		t.Fatalf("results ctrl+n/p: cursor %d", a.search.cursor)
	}
}

func TestSearchEnterSameQueryFocusesResults(t *testing.T) {
	a, be := reopened(t)
	before := len(be.searches())
	drive(a, key("enter"))
	if len(be.searches()) != before {
		t.Fatalf("enter re-ran the search: %+v", be.searches())
	}
	if a.search.focusInput || !a.search.visible {
		t.Fatalf("focus %v visible %v", a.search.focusInput, a.search.visible)
	}
	drive(a, key("j"))
	if a.search.cursor != 1 || a.search.input.Value() != "soul" {
		t.Fatalf("cursor %d query %q", a.search.cursor, a.search.input.Value())
	}
	// A changed query still searches.
	before = len(be.searches())
	drive(a, key("/"), key("s"), key("enter"))
	if len(be.searches()) != before+1 {
		t.Fatalf("changed query: %+v", be.searches())
	}
}

func TestSearchClickSelectsThenOpens(t *testing.T) {
	a, _ := reopened(t)
	var opened []string
	a.opener = func(u string) error { opened = append(opened, u); return nil }
	r := a.overlayRect(a.search.View())
	row := func(i int) tea.Msg { return click(r.x+4, r.y+searchResultsY+i*rowLines) }
	drive(a, row(1))
	if a.search.cursor != 1 || a.search.focusInput || !a.search.visible || a.ref != "bg/2/13" {
		t.Fatalf("cursor %d focus %v visible %v ref %q", a.search.cursor, a.search.focusInput, a.search.visible, a.ref)
	}
	drive(a, key("j"))
	if a.search.cursor != 2 || a.search.input.Value() != "soul" {
		t.Fatalf("j: cursor %d query %q", a.search.cursor, a.search.input.Value())
	}
	drive(a, row(2))
	if a.search.visible || a.ref != a.search.results[2].Ref {
		t.Fatalf("second click: visible %v ref %q", a.search.visible, a.ref)
	}
	if len(opened) != 0 {
		t.Fatalf("browser opened: %v", opened)
	}
}

func TestSearchShiftTabCyclesBack(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("shift+tab"))
	if a.search.scope != 3 {
		t.Fatalf("scope %d, want 3 (lectures)", a.search.scope)
	}
	drive(a, key("shift+tab"), key("shift+tab"))
	if a.search.scope != 1 {
		t.Fatalf("scope %d, want 1", a.search.scope)
	}
	drive(a, key("tab"))
	if a.search.scope != 2 {
		t.Fatalf("tab: scope %d", a.search.scope)
	}
}

func TestSearchHints(t *testing.T) {
	a, _ := searchApp(t)
	if v := plain(a); !strings.Contains(v, "enter search · tab scope · esc close") {
		t.Fatalf("no-results hint:\n%s", v)
	}
	drive(a, key("enter"), key("/"))
	if v := plain(a); !strings.Contains(v, "enter search · ctrl-n/↓ results · tab scope · esc close") {
		t.Fatalf("box hint:\n%s", v)
	}
	drive(a, key("down"))
	if v := plain(a); !strings.Contains(v, "enter open · j/k move · / edit query · tab scope · esc close") {
		t.Fatalf("results hint:\n%s", v)
	}
}

func TestSearchArrowsInEmptyBoxStayInBox(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("/"), key("down"), key("ctrl+n"), key("j"))
	if !a.search.focusInput || a.search.input.Value() != "j" {
		t.Fatalf("focus %v query %q", a.search.focusInput, a.search.input.Value())
	}
}

// The API sends the translation twice: plain and as a highlighted snippet. A
// match only in the translation must not print the same sentence twice.
func TestSearchRowNoDuplicateTranslation(t *testing.T) {
	s := NewSearch(render.Default)
	tr := "As usual for small children, He learned to play."
	s.results = []api.SearchResult{
		{Ref: "cc/adi/14/40", Translation: tr, SnippetTranslation: "As usual for small <mark>children</mark>, He learned to play."},
		{Ref: "sb/6/5/24", Translation: "Dakṣa lamented his lost children.",
			SnippetTranslation: "Dakṣa lamented his lost <mark>children</mark>.",
			SnippetPurport:     "expert in begetting <mark>children</mark>."},
		{Ref: "sb/4/23/33", Translation: "Anyone who hears this narration.",
			SnippetTranslation: "…Anyone who hears this narration.",
			SnippetPurport:     "…surrounded by many <mark>children</mark>."},
	}
	text := func(i int) []string {
		lines := s.row(i, 100)
		for j := range lines {
			lines[j] = strings.TrimSpace(ansiRe.ReplaceAllString(lines[j], ""))
		}
		return lines
	}

	got := text(0)
	if len(got) != rowLines || got[1] != tr || got[2] != "" {
		t.Errorf("translation-only match: %q", got)
	}
	if !strings.Contains(s.row(0, 100)[1], s.hl.Render("children")) {
		t.Error("translation line not highlighted")
	}
	got = text(1)
	if got[1] != "Dakṣa lamented his lost children." || got[2] != "expert in begetting children." {
		t.Errorf("both matched: %q", got)
	}
	got = text(2) // no mark in the translation snippet: show the full translation
	if got[1] != "Anyone who hears this narration." || got[2] != "…surrounded by many children." {
		t.Errorf("purport-only match: %q", got)
	}
}

func TestSearchResultsOfOtherLanguageDropped(t *testing.T) {
	a, _ := searchApp(t)
	drive(a, key("enter"))
	n := len(a.search.results)
	drive(a, searchResultsMsg{q: "soul", gen: a.search.gen, req: 2, lang: "es",
		page: api.SearchPage{Results: []api.SearchResult{{Ref: "bg/9/9", Book: "bg"}}}})
	if len(a.search.results) != n {
		t.Fatalf("results grew from %d to %d", n, len(a.search.results))
	}
}
