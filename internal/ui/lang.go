package ui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
)

// langNames are the supported text languages, in the picker's order, with the
// name each is shown by.
var langNames = []struct{ code, name string }{
	{"en", "English"}, {"es", "Español"}, {"pt", "Português"}, {"hi", "Hindī"}, {"ru", "Русский"},
}

func validLang(code string) bool {
	for _, l := range langNames {
		if l.code == code {
			return true
		}
	}
	return false
}

func langName(code string) string {
	for _, l := range langNames {
		if l.code == code {
			return l.name
		}
	}
	return code
}

// isEnglishOnly reports whether ref is a letter or a transcript, which exist
// in English only and are read as such under any language.
func isEnglishOnly(ref string) bool {
	return strings.HasPrefix(ref, "letters/") || strings.HasPrefix(ref, "transcripts/") ||
		ref == "letters" || ref == "transcripts"
}

// searchLang is the language a search in scope (a book code, or "") is sent in.
func searchLang(scope, lang string) string {
	if scope == "letters" || scope == "transcripts" {
		return "en"
	}
	return lang
}

// available reports whether ref's book is read in the current language. A
// book the books list has not told us about yet counts as available.
func (a *App) available(ref string) bool {
	if a.lang == "en" || isEnglishOnly(ref) {
		return true
	}
	code, _, _ := strings.Cut(ref, "/")
	langs, ok := a.bookLangs[code]
	return !ok || slices.Contains(langs, a.lang)
}

// unavailableNote is shown in the reader instead of an item the current
// language does not have; there is no fallback to English.
func (a *App) unavailableNote() string {
	return "This book isn't available in " + langName(a.lang) + " on vedabase.cc."
}

// loadBooks fetches the books list for the languages each book has.
func (a *App) loadBooks() tea.Cmd {
	be := a.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		books, err := be.Books(ctx, "en")
		if err != nil {
			return nil // every book counts as available until the list is known
		}
		return booksMsg{books: books}
	}
}

func (a *App) onBooks(m booksMsg) {
	a.bookLangs = make(map[string][]string, len(m.books))
	for _, b := range m.books {
		a.bookLangs[b.Code] = b.Languages
	}
	for code := range a.tocPending { // books this language lacks have no contents to fetch
		if !a.available(code) {
			delete(a.tocPending, code)
		}
	}
}

// SetLang switches the text language: the contents of the new language are
// applied (the rest queued), the open item reloads in it at the top, and
// anything still in flight for the old one is dropped when it arrives.
func (a *App) SetLang(code string) tea.Cmd {
	if code == a.lang || !validLang(code) {
		return nil
	}
	a.lang = code
	a.state.Lang = code
	a.persist()

	// Contents, learned titles, prefetches and renders belong to one language.
	// A TOC fetch already running keeps tocBusy until it returns; onTOC
	// ignores what it brings.
	a.tocs = map[string]api.TOC{}
	a.tocPending = map[string]bool{}
	a.tocFailed = map[string]bool{}
	a.learn.titles = map[string]string{}
	a.prefetched = map[string]bool{}
	a.search.langChanged()

	var cmds []tea.Cmd
	if a.lib != nil {
		// ApplyTOC replaces a book's tree in place and cannot undo it, so the
		// library is built again from the cached sitemap, which is cheap.
		cmds = append(cmds, a.resetLibrary(code))
	}
	if a.ref != "" {
		fwd := a.fwd
		a.restoreRef = ""
		cmds = append(cmds, a.open(a.ref, a.node))
		a.fwd = fwd
	}
	return tea.Batch(cmds...)
}

func (a *App) resetLibrary(lang string) tea.Cmd {
	be := a.be
	return func() tea.Msg {
		lib, _, err := be.Library(context.Background(), nil)
		return libraryMsg{lib: lib, err: err, reset: true, lang: lang}
	}
}

// onLibraryReset swaps in the library rebuilt for a language switch.
func (a *App) onLibraryReset(m libraryMsg) tea.Cmd {
	if m.lang != a.lang || m.err != nil || m.lib == nil || a.lib == nil {
		return nil
	}
	a.lib = m.lib
	a.colViews = nil
	a.setCollections()
	a.loadCachedTOCs(m.lib)
	a.applyLearned()
	a.refindNode()
	a.layout()
	return a.nextTOC()
}
