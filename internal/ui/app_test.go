package ui

import (
	"context"
	"encoding/json"
	"errors"
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
	itemLang []string // language of each Item call, parallel to itemCall
	books    []api.Book
	libCalls int
	refresh  int

	refreshCalls []string  // Refresh(ref)
	refreshItem  *api.Item // what Refresh returns; nil: the unchanged item
	refreshErr   error
	expired      map[string]bool // Expired(ref) serves the item for these

	yearList  []api.YearCount // LetterYears serves these
	yearsErr  error
	yearPages map[[2]int]api.Listing // LetterYearPage(year, page); a missing page fails
	letterReq []string               // "years" and "<year>/<page>", in call order

	searchCalls []searchCall
	searchLangs []string // language of each Search call, parallel to searchCalls
	searchErr   error

	cachedTOC map[string]api.TOC // CachedTOC serves these (disk)
	tocFresh  bool               // ...and calls them fresh
	tocs      map[string]api.TOC // TOC serves these; any other book fails
	tocCalls  []string
	onTOC     func(code string) // runs inside TOC, before it answers
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

func (b *fakeBackend) Item(ctx context.Context, ref, lang string) (api.Item, bool, error) {
	b.mu.Lock()
	b.itemCall = append(b.itemCall, ref)
	b.itemLang = append(b.itemLang, lang)
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

func (b *fakeBackend) Refresh(ctx context.Context, ref, lang string) (api.Item, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refreshCalls = append(b.refreshCalls, ref)
	if b.refreshErr != nil {
		return api.Item{}, false, b.refreshErr
	}
	if b.refreshItem != nil {
		return *b.refreshItem, true, nil
	}
	return b.item(ref), false, nil
}

func (b *fakeBackend) Expired(ref, lang string) (api.Item, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.expired[ref] {
		return api.Item{}, false
	}
	return b.item(ref), true
}

func (b *fakeBackend) Cached(ref, lang string) bool {
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
func (b *fakeBackend) Search(ctx context.Context, q, book string, page int, lang string) (api.SearchPage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.searchCalls = append(b.searchCalls, searchCall{q, book, page})
	b.searchLangs = append(b.searchLangs, lang)
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

func (b *fakeBackend) Books(ctx context.Context, lang string) ([]api.Book, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.books, nil
}

func (b *fakeBackend) TOC(ctx context.Context, code, lang string) (api.TOC, error) {
	b.mu.Lock()
	b.tocCalls = append(b.tocCalls, code)
	hook := b.onTOC
	b.mu.Unlock()
	if hook != nil {
		hook(code)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if t, ok := b.tocs[code]; ok {
		return t, nil
	}
	return api.TOC{}, errors.New("no toc for " + code)
}

func (b *fakeBackend) CachedTOC(code, lang string) (api.TOC, bool, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.cachedTOC[code]
	return t, b.tocFresh, ok
}

func (b *fakeBackend) tocCalled() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.tocCalls...)
}

func (b *fakeBackend) searches() []searchCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]searchCall(nil), b.searchCalls...)
}

func (b *fakeBackend) LetterYears(ctx context.Context) ([]api.YearCount, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.letterReq = append(b.letterReq, "years")
	if b.yearsErr != nil {
		return nil, 0, b.yearsErr
	}
	return b.yearList, 57, nil
}

func (b *fakeBackend) LetterYearPage(ctx context.Context, year, page int) (api.Listing, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.letterReq = append(b.letterReq, fmt.Sprintf("%d/%d", year, page))
	l, ok := b.yearPages[[2]int{year, page}]
	if !ok {
		return api.Listing{}, errors.New("no such page")
	}
	return l, nil
}

func (b *fakeBackend) lettersCalled() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.letterReq...)
}

// lettersFixtures serves 1967 as two pages: 20 letters, then 5 more. The
// first two and the last are in the sitemap's By-recipient list too.
func (b *fakeBackend) lettersFixtures() {
	b.yearList = []api.YearCount{{Year: 1966, Count: 1}, {Year: 1967, Count: 25}}
	entry := func(ref, title string) api.ListingEntry {
		return api.ListingEntry{Ref: ref, Title: title, IsoDate: "1967-01-01", Year: 1967}
	}
	p1 := []api.ListingEntry{entry("letters/letter-to-aatie", "Letter to: Aatie"), entry("letters/letter-to-abhirama", "Letter to: Abhirama")}
	for i := range 18 {
		p1 = append(p1, entry(fmt.Sprintf("letters/letter-to-x%d", i), fmt.Sprintf("Letter to: X%d", i)))
	}
	var p2 []api.ListingEntry
	for i := range 5 {
		p2 = append(p2, entry(fmt.Sprintf("letters/letter-to-y%d", i), fmt.Sprintf("Letter to: Y%d", i)))
	}
	b.yearPages = map[[2]int]api.Listing{
		{1967, 1}: {Total: 25, Page: 1, PerPage: 100, HasMore: true, Items: p1},
		{1967, 2}: {Total: 25, Page: 2, PerPage: 100, Items: p2},
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
	drive(a, itemLoadedMsg{ref: "bg/1/19", lang: "en", item: be.item("bg/1/19")})
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

func TestItemOfOtherLanguageDropped(t *testing.T) {
	be := newFake(t)
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, a.Open("bg/2/1")())
	drive(a, itemLoadedMsg{ref: "bg/2/1", lang: "es", item: api.Item{Ref: "bg/2/1", Book: "bg", Translation: "OTHERLANG"}})
	if v := plain(a); strings.Contains(v, "OTHERLANG") {
		t.Fatalf("item of another language shown:\n%s", v)
	}
}

func TestStepFollowsSiteOrder(t *testing.T) {
	be := newFake(t)
	it := be.items["bg/2/13"]
	it.Prev, it.Next = "bg/1/19", "bg/2/14"
	be.items["bg/2/13"] = it
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("l"))
	if a.ref != "bg/2/14" || a.hasItem != true {
		t.Fatalf("after l: %q", a.ref)
	}
	if !strings.Contains(plain(a), "Translation of bg/2/14") {
		t.Fatalf("target not shown:\n%s", plain(a))
	}
	a, _ = boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("h"))
	if a.ref != "bg/1/19" || a.node == nil || a.node.Ref != "bg/1/19" {
		t.Fatalf("after h: ref %q node %v", a.ref, a.node)
	}
}

func TestStepFallsBackToLibraryOrder(t *testing.T) {
	be := newFake(t)
	it := be.items["bg/2/13"]
	it.Prev, it.Next = "", ""
	be.items["bg/2/13"] = it
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	want := a.lib.Next(a.node).Ref
	drive(a, key("l"))
	if a.ref != want {
		t.Fatalf("after l: %q, want %q", a.ref, want)
	}
}

func TestPrefetchUsesSiteNext(t *testing.T) {
	be := newFake(t)
	it := be.items["bg/2/13"]
	it.Next = "bg/2/14"
	be.items["bg/2/13"] = it
	boot(t, be, store.State{LastRef: "bg/2/13"})
	if !be.called("bg/2/14") {
		t.Fatalf("calls = %v", be.itemCall)
	}
}
