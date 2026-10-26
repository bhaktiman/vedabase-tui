package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/render"
	"vedabase-tui/internal/store"
)

// langFake is a backend where bg is read in en/es/pt and sb in English only.
func langFake(t *testing.T) *fakeBackend {
	t.Helper()
	be := newFake(t)
	be.books = []api.Book{
		{Code: "bg", Title: "Bhagavad-gītā As It Is", Type: "chapters", Languages: []string{"en", "es", "pt"}},
		{Code: "sb", Title: "Śrīmad-Bhāgavatam", Type: "cantos", Languages: []string{"en"}},
	}
	return be
}

// setLang switches the language and runs what that starts.
func setLang(a *App, code string) {
	if cmd := a.SetLang(code); cmd != nil {
		drive(a, cmd())
	}
}

func (b *fakeBackend) itemReqs() (refs, langs []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.itemCall...), append([]string(nil), b.itemLang...)
}

func (b *fakeBackend) lastItemReq() (string, string) {
	refs, langs := b.itemReqs()
	if len(refs) == 0 {
		return "", ""
	}
	return refs[len(refs)-1], langs[len(langs)-1]
}

func TestStateLangValidated(t *testing.T) {
	for in, want := range map[string]string{"": "en", "es": "es", "hi": "hi", "xx": "en", "EN": "en"} {
		a, _ := boot(t, langFake(t), store.State{Lang: in})
		if a.lang != want {
			t.Errorf("state lang %q -> %q, want %q", in, a.lang, want)
		}
	}
}

func TestSetLangReopensItemInNewLanguage(t *testing.T) {
	be := langFake(t)
	a, sv := boot(t, be, store.State{LastRef: "bg/2/13", Scroll: 3})
	drive(a, key("j"), key("j"))
	setLang(a, "es")
	if r, l := be.lastItemReq(); r != "bg/2/13" || l != "es" {
		t.Fatalf("last item request = %s %s, want bg/2/13 es", r, l)
	}
	if a.reader.offset != 0 {
		t.Fatalf("offset = %d, want the top", a.reader.offset)
	}
	if got := sv.calls[len(sv.calls)-1].Lang; got != "es" {
		t.Fatalf("saved lang = %q", got)
	}
	if a.SetLang("es") != nil || a.SetLang("xx") != nil {
		t.Fatal("same or invalid language should be a no-op")
	}
	if a.lang != "es" {
		t.Fatalf("lang = %q", a.lang)
	}
}

func TestSetLangAppliesOnDiskTOCsAndDropsOld(t *testing.T) {
	be := langFake(t)
	be.cachedTOC = map[string]api.TOC{"bg": smallBG(true)}
	be.tocFresh = true
	a := bootWith(t, be, "bg/2/13")
	if a.tocs["bg"].Title == "" {
		t.Fatal("the English contents were not applied")
	}
	be.cachedTOC = nil // nothing on disk for es
	be.tocFresh = false
	be.lib = sampleLibrary(t) // the service parses the cached sitemap afresh
	before := len(be.tocCalled())
	setLang(a, "es")
	if _, ok := a.tocs["bg"]; ok {
		t.Fatal("English contents survived the switch")
	}
	if calls := be.tocCalled()[before:]; len(calls) == 0 || calls[0] != "bg" {
		t.Fatalf("bg, the open book, should be fetched first for es: %v", calls)
	}
	if n := a.lib.Find("bg/1"); n != nil && strings.Contains(n.Title, "From the API") {
		t.Fatal("the English book tree is still in the library")
	}
}

func TestBookWithoutLanguageShowsNoteAndNeverAsksEnglish(t *testing.T) {
	be := langFake(t)
	a, _ := boot(t, be, store.State{LastRef: "sb/1/1/1"})
	n := len(be.itemCall)
	setLang(a, "es")
	refs, langs := be.itemReqs()
	for i := n; i < len(refs); i++ {
		if refs[i] == "sb/1/1/1" {
			t.Fatalf("sb requested after the switch in %q", langs[i])
		}
	}
	if v := plain(a); !strings.Contains(v, "This book isn't available in Español on vedabase.cc.") {
		t.Fatalf("no note:\n%s", v)
	}
	if a.toasts.cur != nil {
		t.Fatalf("toast = %+v", a.toasts.cur)
	}
	// stepping within the book neither fetches nor prefetches it
	n = len(be.itemCall)
	drive(a, key("l"))
	refs, langs = be.itemReqs()
	for i := n; i < len(refs); i++ {
		if strings.HasPrefix(refs[i], "sb/") {
			t.Fatalf("sb requested in %q after l", langs[i])
		}
	}
	// back to English reads it again
	setLang(a, "en")
	if r, l := be.lastItemReq(); !strings.HasPrefix(r, "sb/") || l != "en" {
		t.Fatalf("last request %s %s", r, l)
	}
}

func TestStartupInUnavailableLanguageShowsNote(t *testing.T) {
	be := langFake(t)
	a, _ := boot(t, be, store.State{LastRef: "sb/1/1/1", Lang: "pt"})
	for i, r := range be.itemCall {
		if strings.HasPrefix(r, "sb/") {
			t.Fatalf("sb requested in %q", be.itemLang[i])
		}
	}
	if v := plain(a); !strings.Contains(v, "Português") {
		t.Fatalf("no note:\n%s", v)
	}
}

func TestNotFoundInLanguageShowsNote(t *testing.T) {
	be := langFake(t)
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	be.itemErr["bg/2/14"] = &api.APIError{Status: 404, Code: "not_found"}
	setLang(a, "pt")
	drive(a, a.Open("bg/2/14")())
	if v := plain(a); !strings.Contains(v, "This book isn't available in Português on vedabase.cc.") {
		t.Fatalf("no note:\n%s", v)
	}
	if a.toasts.cur != nil {
		t.Fatalf("toast = %+v", a.toasts.cur)
	}
	for i, r := range be.itemCall {
		if r == "bg/2/14" && be.itemLang[i] != "pt" {
			t.Fatalf("asked for it in %q", be.itemLang[i])
		}
	}
}

func TestLettersStayEnglishUnderAnyLanguage(t *testing.T) {
	be := langFake(t)
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13", Lang: "es"})
	drive(a, a.Open("letters/letter-to-aatie")())
	if r, l := be.lastItemReq(); r != "letters/letter-to-aatie" || l != "en" {
		t.Fatalf("last request %s %s", r, l)
	}
	if v := plain(a); strings.Contains(v, "isn't available") {
		t.Fatalf("letters got the note:\n%s", v)
	}
}

func TestSearchLanguage(t *testing.T) {
	a, be := searchApp(t)
	setLang(a, "es")
	drive(a, key("/"), key("enter"))
	drive(a, key("/"), key("tab")) // this book
	drive(a, key("/"), key("tab")) // letters
	drive(a, key("/"), key("tab")) // lectures
	be.mu.Lock()
	got := append([]string(nil), be.searchLangs...)
	calls := append([]searchCall(nil), be.searchCalls...)
	be.mu.Unlock()
	if len(got) != 4 {
		t.Fatalf("searches = %+v", calls)
	}
	want := []string{"es", "es", "en", "en"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("search %d (%q) lang = %q, want %q", i, calls[i].book, got[i], want[i])
		}
	}
	if len(a.search.results) == 0 {
		t.Fatal("letters/lectures results (en) were dropped as stale")
	}
}

func TestSetLangUnsticksSearchInFlight(t *testing.T) {
	a, _ := searchApp(t)
	cmd := a.search.run() // a search is in flight and its reply is never delivered
	_ = cmd
	if !a.search.loading {
		t.Fatal("setup: not loading")
	}
	setLang(a, "es")
	if a.search.loading {
		t.Fatal("search still loading after the switch")
	}
	// the old reply, late, changes nothing
	drive(a, searchResultsMsg{q: "soul", page: api.SearchPage{Results: []api.SearchResult{{Ref: "bg/2/13"}}}, gen: a.search.gen, req: 1, lang: "en"})
	if len(a.search.results) != 0 {
		t.Fatal("old-language results shown")
	}
	// a new query goes out in the new language
	if !a.search.visible || !a.search.focusInput {
		t.Fatal("the overlay should stay open on its query")
	}
	drive(a, key("enter"))
	if a.search.q != "soul" || !a.search.live {
		t.Fatalf("query not re-run: q=%q live=%v", a.search.q, a.search.live)
	}
}

func TestLateTOCFromOldLanguageKeepsNewBookPending(t *testing.T) {
	be := langFake(t)
	a := bootWith(t, be, "bg/2/13")
	a.tocBusy = true // an English fetch of bg is in flight
	delete(a.tocPending, "bg")
	setLang(a, "es")
	if !a.tocPending["bg"] {
		t.Fatal("setup: bg should be pending for es")
	}
	drive(a, tocMsg{code: "bg", lang: "en", toc: smallBG(true)})
	if a.tocBusy && len(be.tocCalled()) == 0 {
		t.Fatal("tocBusy not released")
	}
	if _, ok := a.tocs["bg"]; ok && a.tocs["bg"].Lang == "en" {
		t.Fatal("English contents applied under es")
	}
	// the queue went on with bg in es
	found := false
	for _, c := range be.tocCalled() {
		found = found || c == "bg"
	}
	if !found && !a.tocPending["bg"] {
		t.Fatal("the new language's pending entry for bg was cleared without a fetch")
	}
}

func TestRenderCacheKeepsLanguagesApart(t *testing.T) {
	r := render.NewRenderer(render.Default)
	if _, _, err := r.Render("bg/2/13", "en", "english", nil, 40, api.AllParts()); err != nil {
		t.Fatal(err)
	}
	out, _, _ := r.Render("bg/2/13", "es", "español", nil, 40, api.AllParts())
	if !strings.Contains(out, "español") {
		t.Fatalf("served the English render: %q", out)
	}
}

func TestStaleLanguageRenderDropped(t *testing.T) {
	a, _ := boot(t, langFake(t), store.State{LastRef: "bg/2/13"})
	setLang(a, "es")
	drive(a, renderedMsg{ref: a.ref, lang: "en", width: a.textW(), parts: a.parts, pal: a.pal.Name, out: "STALELANG"})
	if v := plain(a); strings.Contains(v, "STALELANG") {
		t.Fatalf("stale render shown:\n%s", v)
	}
}

func TestExportFrontMatterHasLang(t *testing.T) {
	be := langFake(t)
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13", Lang: "es"})
	dir := t.TempDir()
	a.settings.ExportDir = dir
	drive(a, a.exportItem("bg/2/13")())
	b, err := os.ReadFile(filepath.Join(dir, "bg-2-13.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\nlang: \"es\"\n") {
		t.Fatalf("front matter:\n%s", b)
	}
	drive(a, a.exportItem("letters/letter-to-aatie")())
	b, err = os.ReadFile(filepath.Join(dir, "letters-letter-to-aatie.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\nlang: \"en\"\n") {
		t.Fatalf("letter front matter:\n%s", b)
	}
}
