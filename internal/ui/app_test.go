package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/store"
)

// fakeBackend serves the sample library and synthetic items, and records calls.
type fakeBackend struct {
	t *testing.T

	mu       sync.Mutex
	lib      *library.Library
	items    map[string]api.Item
	cached   map[string]bool
	itemErr  map[string]error
	libErr   error
	stale    bool
	block    map[string]chan struct{} // Item(ref) waits for close
	itemCall []string
	libCalls int
	refresh  int

	pages     map[string][]byte // Page(url) fixtures
	pageErr   error
	pageCalls []string

	searchCalls []searchCall
	searchErr   error
}

type searchCall struct {
	q, book string
	page    int
}

func newFake(t *testing.T) *fakeBackend {
	t.Helper()
	b := &fakeBackend{t: t, lib: sampleLibrary(t), items: map[string]api.Item{}, cached: map[string]bool{},
		itemErr: map[string]error{}, block: map[string]chan struct{}{}}
	data, err := os.ReadFile("../api/testdata/verse-bg-2-13.json")
	if err != nil {
		t.Fatal(err)
	}
	var it api.Item
	if err := json.Unmarshal(data, &it); err != nil {
		t.Fatal(err)
	}
	b.items["bg/2/13"] = it
	return b
}

func (b *fakeBackend) item(ref string) api.Item {
	if it, ok := b.items[ref]; ok {
		return it
	}
	return api.Item{Ref: ref, Book: "bg", VerseText: "> verse of " + ref, Synonyms: "*words*—of " + ref,
		Translation: "**Translation of " + ref + "**", Purport: "Purport of " + ref}
}

func (b *fakeBackend) Item(ctx context.Context, ref string) (api.Item, bool, error) {
	b.mu.Lock()
	b.itemCall = append(b.itemCall, ref)
	ch, err := b.block[ref], b.itemErr[ref]
	b.mu.Unlock()
	if ch != nil {
		<-ch
	}
	if err != nil {
		return api.Item{}, false, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	was := b.cached[ref]
	b.cached[ref] = true
	return b.item(ref), was, nil
}

func (b *fakeBackend) Cached(ref string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cached[ref]
}

func (b *fakeBackend) Library(ctx context.Context, progress func(done, total int64)) (*library.Library, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.libCalls++
	if b.libErr != nil {
		return nil, false, b.libErr
	}
	return b.lib, b.stale, nil
}

func (b *fakeBackend) RefreshLibrary(ctx context.Context) (*library.Library, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refresh++
	return b.lib, nil
}

// Search serves a two-page result set: page 1 has bg/2/13 and bg/2/14, page 2
// has bg/2/15.
func (b *fakeBackend) Search(ctx context.Context, q, book string, page int) (api.SearchPage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.searchCalls = append(b.searchCalls, searchCall{q, book, page})
	if b.searchErr != nil {
		return api.SearchPage{}, b.searchErr
	}
	res := func(ref string) api.SearchResult {
		return api.SearchResult{Ref: ref, Book: "bg", BookTitle: "Bhagavad-gītā As It Is",
			Translation:    "Translation line of " + ref,
			SnippetPurport: "the <mark>" + q + "</mark> is eternal in " + ref}
	}
	if page == 1 {
		return api.SearchPage{Query: q, Book: book, Page: 1, HasMore: true,
			Results: []api.SearchResult{res("bg/2/13"), res("bg/2/14")}}, nil
	}
	return api.SearchPage{Query: q, Book: book, Page: page, Results: []api.SearchResult{res("bg/2/15")}}, nil
}

func (b *fakeBackend) searches() []searchCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]searchCall(nil), b.searchCalls...)
}

func (b *fakeBackend) Page(ctx context.Context, url string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pageCalls = append(b.pageCalls, url)
	if b.pageErr != nil {
		return nil, b.pageErr
	}
	return b.pages[url], nil
}

func (b *fakeBackend) pagesCalled() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.pageCalls...)
}

// scrapeFixtures serves the recorded site pages.
func (b *fakeBackend) scrapeFixtures() {
	b.t.Helper()
	b.pages = map[string][]byte{}
	for url, f := range map[string]string{
		"https://vedabase.cc/bg/":                                  "bg.html",
		"https://vedabase.cc/en/library/letters/":                  "letters.html",
		"https://vedabase.cc/en/library/letters/?year=1967":        "letters-1967-p1.html",
		"https://vedabase.cc/en/library/letters/?year=1967&page=2": "letters-1967-p1.html",
	} {
		data, err := os.ReadFile("../scrape/testdata/" + f)
		if err != nil {
			b.t.Fatal(err)
		}
		b.pages[url] = data
	}
}

func (b *fakeBackend) called(ref string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.itemCall {
		if r == ref {
			return true
		}
	}
	return false
}

// drive feeds msgs to a and runs the Cmds that come back, feeding their
// messages in too. Timers do not fire: the app's tick is stubbed in newApp.
func drive(a *App, msgs ...tea.Msg) *App {
	queue := msgs
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		switch m := m.(type) {
		case nil:
			continue
		case tea.BatchMsg:
			for _, c := range m {
				if c != nil {
					queue = append(queue, c())
				}
			}
			continue
		case tea.QuitMsg:
			continue
		}
		if _, cmd := a.Update(m); cmd != nil {
			queue = append(queue, cmd())
		}
	}
	return a
}

type saved struct {
	mu    sync.Mutex
	calls []store.State
}

func (s *saved) save(st store.State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, st)
	return nil
}

// boot starts an app on a 120x40 terminal and runs it until it settles.
func boot(t *testing.T, be *fakeBackend, st store.State) (*App, *saved) {
	t.Helper()
	sv := &saved{}
	if st.Parts == (api.Parts{}) {
		st.Parts = api.AllParts()
	}
	a := New(Options{Backend: be, State: st, SaveState: sv.save})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	return a, sv
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;:]*[A-Za-z]`)

func plain(a *App) string { return ansiRe.ReplaceAllString(a.View().Content, "") }

func TestStartOpensLastRef(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	v := plain(a)
	for _, want := range []string{"As the embodied soul", "Chapter 2", "2 / 2"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
}

func TestStartsAtFirstGitaLeafWithoutState(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{})
	if a.ref != "bg/0a" {
		t.Fatalf("ref = %q", a.ref)
	}
}

func TestRestoresScroll(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13", Scroll: 3})
	if a.reader.offset != 3 {
		t.Fatalf("offset = %d", a.reader.offset)
	}
}

func TestNextPrevCrossesChapter(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/1/19"})
	drive(a, key("l"))
	if a.ref != "bg/2/1" {
		t.Fatalf("after l: %q", a.ref)
	}
	drive(a, key("h"))
	if a.ref != "bg/1/19" {
		t.Fatalf("after h: %q", a.ref)
	}
}

func TestEndOfBookToast(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/10/1"})
	drive(a, key("l"))
	if a.ref != "bg/10/1" {
		t.Fatalf("ref moved to %q", a.ref)
	}
	if a.toasts.cur == nil || a.toasts.cur.text != "End of Bhagavad-gītā As It Is" {
		t.Fatalf("toast = %+v", a.toasts.cur)
	}
	if !strings.Contains(plain(a), "End of Bhagavad-gītā As It Is") {
		t.Fatal("toast not drawn")
	}
}

func TestTogglePartsPersist(t *testing.T) {
	a, sv := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if !strings.Contains(plain(a), "dehinaḥ") {
		t.Fatalf("word-for-word missing before toggle:\n%s", plain(a))
	}
	drive(a, key("3"))
	if strings.Contains(plain(a), "dehinaḥ") {
		t.Fatal("word-for-word still shown")
	}
	drive(a, key("q"))
	// State is also saved on open and on each toggle; q writes the last one.
	got := sv.calls[len(sv.calls)-1]
	if got.Parts.Synonyms || !got.Parts.Translation || got.LastRef != "bg/2/13" {
		t.Fatalf("saved %+v", got)
	}
}

func TestStaleResponseDropped(t *testing.T) {
	be := newFake(t)
	be.block["bg/1/19"] = make(chan struct{})
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	_ = a.Open("bg/1/19")        // slow A; its command is never run
	drive(a, a.Open("bg/2/1")()) // fast B
	drive(a, itemLoadedMsg{ref: "bg/1/19", item: be.item("bg/1/19")})
	v := plain(a)
	if !strings.Contains(v, "Translation of bg/2/1") || strings.Contains(v, "Translation of bg/1/19") {
		t.Fatalf("reader:\n%s", v)
	}
}

func TestStaleRenderDropped(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, renderedMsg{ref: a.ref, width: a.readerW() + 7, parts: a.parts, pal: a.pal.Name, out: "STALEWIDTH"})
	p := a.parts
	p.Purport = !p.Purport
	drive(a, renderedMsg{ref: a.ref, width: a.readerW(), parts: p, pal: a.pal.Name, out: "STALEPARTS"})
	if v := plain(a); strings.Contains(v, "STALE") {
		t.Fatalf("stale render shown:\n%s", v)
	}
}

func TestPrefetchNext(t *testing.T) {
	be := newFake(t)
	boot(t, be, store.State{LastRef: "bg/1/16-18"})
	if !be.called("bg/1/19") {
		t.Fatalf("calls = %v", be.itemCall)
	}
}

func TestOfflineMessages(t *testing.T) {
	be := newFake(t)
	be.libErr = api.ErrOffline
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	if !strings.Contains(plain(a), "Can't reach vedabase.cc") {
		t.Fatalf("view:\n%s", plain(a))
	}
	be.libErr = nil
	drive(a, key("r"))
	if be.libCalls != 2 || a.ref != "bg/2/13" {
		t.Fatalf("libCalls=%d ref=%q", be.libCalls, a.ref)
	}

	be2 := newFake(t)
	be2.itemErr["bg/2/13"] = fmt.Errorf("%w: dial", api.ErrOffline)
	a2, _ := boot(t, be2, store.State{LastRef: "bg/2/13"})
	if !strings.Contains(plain(a2), "You're offline") {
		t.Fatalf("view:\n%s", plain(a2))
	}
}

func TestRateLimitedMessage(t *testing.T) {
	be := newFake(t)
	be.itemErr["bg/2/13"] = &api.RateLimitedError{RetryAfter: 12 * time.Second}
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	if !strings.Contains(plain(a), "vedabase.cc asked us to wait 12s") {
		t.Fatalf("view:\n%s", plain(a))
	}
}

func TestOtherErrorToastsAndShows(t *testing.T) {
	be := newFake(t)
	be.itemErr["bg/2/13"] = fmt.Errorf("boom")
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	if a.toasts.cur == nil || !a.toasts.cur.err || !strings.Contains(plain(a), "boom") {
		t.Fatalf("toast=%+v", a.toasts.cur)
	}
}

func TestNarrowTerminal(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, tea.WindowSizeMsg{Width: 30, Height: 8})
	if a.treeVisible() {
		t.Fatal("tree visible at 30 columns")
	}
	_ = a.View()
	drive(a, a.renderCmd()())
	drive(a, key("j"))
	if a.reader.offset != 1 {
		t.Fatalf("offset = %d", a.reader.offset)
	}
	_ = a.View()
}

func TestTreeToggleAndFocus(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if !strings.Contains(plain(a), "▸") {
		t.Fatal("tree not shown")
	}
	drive(a, key("t"))
	if strings.Contains(plain(a), "▸") {
		t.Fatal("tree still shown")
	}
	drive(a, key("t"), key("tab"))
	if !a.focusTree {
		t.Fatal("focus not on tree")
	}
	before := a.tree.Selected()
	drive(a, key("j"))
	if a.tree.Selected() == before || a.reader.offset != 0 {
		t.Fatalf("tree cursor %v -> %v, reader offset %d", before.Ref, a.tree.Selected().Ref, a.reader.offset)
	}
	drive(a, key("tab"), key("j"))
	if a.reader.offset != 1 {
		t.Fatalf("reader offset = %d", a.reader.offset)
	}
}

func TestOpenFromTreeFollowsAndFocusesReader(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("tab"), key("k"), key("enter"))
	if a.ref != "bg/2/1" || a.focusTree {
		t.Fatalf("ref=%q focusTree=%v", a.ref, a.focusTree)
	}
}

func TestResizeIsDebounced(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	var delays []time.Duration
	a.tick = func(d time.Duration, m tea.Msg) tea.Cmd {
		delays = append(delays, d)
		return func() tea.Msg { return m }
	}
	drive(a, tea.WindowSizeMsg{Width: 100, Height: 40}, tea.WindowSizeMsg{Width: 101, Height: 40})
	// The first tick is stale (seq moved on); only the last one renders.
	if len(delays) != 2 || delays[0] != 100*time.Millisecond {
		t.Fatalf("delays = %v", delays)
	}
	if a.renderedW != a.readerW() {
		t.Fatalf("renderedW=%d readerW=%d", a.renderedW, a.readerW())
	}
}

func TestToastsQueueOneAtATime(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.Toast("one", false)
	a.Toast("two", true)
	if a.toasts.cur.text != "one" {
		t.Fatal("first toast not shown")
	}
	a.toastExpired(toastExpiredMsg{a.toasts.seq})
	if a.toasts.cur == nil || a.toasts.cur.text != "two" {
		t.Fatalf("cur = %+v", a.toasts.cur)
	}
	a.toastExpired(toastExpiredMsg{a.toasts.seq})
	if a.toasts.cur != nil {
		t.Fatal("toast should be gone")
	}
}

func TestStaleLibraryRefreshes(t *testing.T) {
	be := newFake(t)
	be.stale = true
	boot(t, be, store.State{LastRef: "bg/2/13"})
	if be.refresh != 1 {
		t.Fatalf("refresh calls = %d", be.refresh)
	}
}
