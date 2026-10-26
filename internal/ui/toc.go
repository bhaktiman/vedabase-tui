package ui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
)

// tocTimeout covers the wait for the TOC limiter plus a big book's download.
const tocTimeout = 60 * time.Second

// tocMsg is the result of one background TOC fetch.
type tocMsg struct {
	code, lang string
	toc        api.TOC
	err        error
}

// isBookRoot reports whether n is the root of a book whose contents the API
// serves: not letters, transcripts or the collections.
func isBookRoot(n *library.Node) bool {
	return n != nil && n.Parent == nil && n.Ref != "letters" && n.Ref != "transcripts" && !strings.Contains(n.Ref, "#")
}

// loadCachedTOCs applies every table of contents on disk to lib and queues the
// books whose copy is missing or old.
func (a *App) loadCachedTOCs(lib *library.Library) {
	for _, r := range lib.Roots {
		if !isBookRoot(r) {
			continue
		}
		t, fresh, ok := a.be.CachedTOC(r.Ref, a.lang)
		if ok && lib.ApplyTOC(r.Ref, t) {
			a.tocs[r.Ref] = t
		} else {
			fresh = false
		}
		if !fresh {
			a.tocPending[r.Ref] = true
		}
	}
}

// reapplyTOCs puts the TOCs we know onto a brand-new library.
func (a *App) reapplyTOCs() {
	for code, t := range a.tocs {
		a.lib.ApplyTOC(code, t)
	}
}

// nextTOCCode picks the next book to fetch: one the reader just opened, the
// current item's book, the expanded books, then the rest in library order.
func (a *App) nextTOCCode() string {
	for _, c := range a.tocFront {
		if a.tocPending[c] {
			return c
		}
	}
	if a.node != nil && a.tocPending[a.node.Book] {
		return a.node.Book
	}
	rest := ""
	for _, r := range a.lib.Roots {
		if !isBookRoot(r) || !a.tocPending[r.Ref] {
			continue
		}
		if a.tree.expanded[r.Ref] {
			return r.Ref
		}
		if rest == "" {
			rest = r.Ref
		}
	}
	return rest
}

// nextTOC starts the next fetch unless one is already running.
func (a *App) nextTOC() tea.Cmd {
	if a.tocBusy || a.lib == nil {
		return nil
	}
	code := a.nextTOCCode()
	if code == "" {
		return nil
	}
	a.tocBusy = true
	be, lang := a.be, a.lang
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), tocTimeout)
		defer cancel()
		t, err := be.TOC(ctx, code, lang)
		return tocMsg{code: code, lang: lang, toc: t, err: err}
	}
}

// bumpTOC moves an expanded book whose contents are not loaded to the front.
func (a *App) bumpTOC(n *library.Node) {
	if !isBookRoot(n) {
		return
	}
	if a.tocFailed[n.Ref] { // the user is asking for it again: one more try
		delete(a.tocFailed, n.Ref)
		a.tocPending[n.Ref] = true
	}
	if !a.tocPending[n.Ref] {
		return
	}
	a.tocFront = append([]string{n.Ref}, slices.DeleteFunc(a.tocFront, func(c string) bool { return c == n.Ref })...)
}

// dropTitled forgets the learned title of every ref the TOC titles itself;
// refs it leaves untitled keep what the item translations taught.
func dropTitled(learned map[string]string, nodes []api.TOCNode) {
	for _, n := range nodes {
		if n.Title != "" {
			delete(learned, n.Ref)
		}
		dropTitled(learned, n.Children)
	}
}

func (a *App) onTOC(m tocMsg) tea.Cmd {
	a.tocBusy = false
	if m.lang != a.lang { // started before a language switch: nothing of it applies
		return a.nextTOC()
	}
	delete(a.tocPending, m.code) // no retry loop: a failure waits until the reader expands the book
	if m.lang == a.lang && m.err != nil {
		a.tocFailed[m.code] = true
	}
	a.tocFront = slices.DeleteFunc(a.tocFront, func(c string) bool { return c == m.code })
	if m.lang == a.lang && m.err == nil && a.lib != nil && a.lib.ApplyTOC(m.code, m.toc) {
		a.tocs[m.code] = m.toc
		// the TOC's titles win over anything scraped for this book
		if m.toc.Title != "" {
			delete(a.learn.titles, m.code)
		}
		dropTitled(a.learn.titles, m.toc.Children)
		a.applyLearned()
		a.refindNode()
	}
	return a.nextTOC()
}
