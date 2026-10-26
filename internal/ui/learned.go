package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
)

const (
	yearRefPfx  = "letters#year/"
	pageTimeout = 30 * time.Second
)

// Results of background fetches. They are applied in Update, never in the Cmd.
type (
	yearsMsg struct {
		years []api.YearCount
		err   error
	}
	yearPageMsg struct {
		year, page int
		entries    []api.ListingEntry
		more       bool
	}
)

type yearPages struct {
	loaded  int // pages applied so far
	more    bool
	loading bool
	refs    []string
	titles  map[string]string
}

// learned remembers what the API told us beyond the sitemap, so it survives
// the library being replaced by a background refresh.
type learned struct {
	titles    map[string]string // node ref -> title
	years     []api.YearCount
	yearsAsk  bool
	yearPages map[int]*yearPages
}

func newLearned() learned {
	return learned{titles: map[string]string{}, yearPages: map[int]*yearPages{}}
}

// onExpandLazy loads the children of a Lazy node: the year list, or a year.
func (a *App) onExpandLazy(n *library.Node) tea.Cmd {
	if n == nil {
		return nil
	}
	switch {
	case n.Ref == "letters#year":
		if a.learn.yearsAsk {
			return nil
		}
		a.learn.yearsAsk = true
		be := a.be
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), pageTimeout)
			defer cancel()
			years, _, err := be.LetterYears(ctx) // undated letters cannot be listed: no filter for them
			return yearsMsg{years: years, err: err}
		}
	case strings.HasPrefix(n.Ref, yearRefPfx):
		y, err := strconv.Atoi(n.Seg)
		if err != nil {
			return nil
		}
		return a.yearPageCmd(y)
	}
	return nil
}

func (a *App) onYears(m yearsMsg) tea.Cmd {
	if m.err != nil || len(m.years) == 0 {
		a.learn.yearsAsk = false // let a later expand retry
		return nil
	}
	a.learn.years = m.years
	a.applyLearned()
	return nil
}

// yearPageCmd fetches the next unloaded page of a year, if there is one and
// none is on its way.
func (a *App) yearPageCmd(y int) tea.Cmd {
	yp := a.learn.yearPages[y]
	if yp == nil {
		yp = &yearPages{more: true, titles: map[string]string{}}
		a.learn.yearPages[y] = yp
	}
	if !yp.more || yp.loading {
		return nil
	}
	yp.loading = true
	page, be := yp.loaded+1, a.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pageTimeout)
		defer cancel()
		l, err := be.LetterYearPage(ctx, y, page)
		if err != nil {
			return yearPageMsg{year: y, page: page}
		}
		return yearPageMsg{year: y, page: page, entries: l.Items, more: l.HasMore}
	}
}

func (a *App) onYearPage(m yearPageMsg) tea.Cmd {
	yp := a.learn.yearPages[m.year]
	if yp == nil {
		return nil
	}
	yp.loading = false
	if len(m.entries) == 0 { // failed or empty: stop, so a failure cannot loop
		yp.more = false
		return nil
	}
	yp.loaded, yp.more = m.page, m.more
	for _, e := range m.entries {
		yp.refs = append(yp.refs, e.Ref)
		yp.titles[e.Ref] = api.SanitizeText(e.Title)
	}
	a.applyLearned()
	return nil
}

// nextYearPage loads the following page when the cursor reaches the last
// loaded letter of a year.
func (a *App) nextYearPage() tea.Cmd {
	n := a.tree.Selected()
	if n == nil || n.Parent == nil || !strings.HasPrefix(n.Parent.Ref, yearRefPfx) {
		return nil
	}
	kids := n.Parent.Children
	if len(kids) == 0 || kids[len(kids)-1] != n {
		return nil
	}
	y, err := strconv.Atoi(n.Parent.Seg)
	if err != nil {
		return nil
	}
	return a.yearPageCmd(y)
}

// yearLetter is the By-year row for ref, looked for in n's year first, so a
// letter read from By year stays under By year.
func (a *App) yearLetter(n *library.Node, ref string) *library.Node {
	if a.lib == nil || n == nil || n.Parent == nil || !strings.HasPrefix(n.Parent.Ref, yearRefPfx) {
		return nil
	}
	groups := []*library.Node{n.Parent}
	if root := a.lib.Find("letters#year"); root != nil {
		groups = append(groups, root.Children...)
	}
	for _, g := range groups {
		for _, c := range g.Children {
			if c.Ref == ref {
				return c
			}
		}
	}
	return nil
}

// applyLearned writes everything learned so far onto the current library and
// refreshes the tree.
func (a *App) applyLearned() {
	if a.lib == nil {
		return
	}
	for _, yc := range a.learn.years {
		var refs []string
		var titles map[string]string
		if yp := a.learn.yearPages[yc.Year]; yp != nil {
			refs, titles = yp.refs, yp.titles
		}
		if g := a.lib.AddYearGroup(yc.Year, refs, titles); g != nil {
			g.Lazy = true
			g.Title = fmt.Sprintf("%d (%d)", yc.Year, yc.Count)
		}
	}
	for r, t := range a.learn.titles {
		if n := a.lib.Find(r); n != nil {
			n.Title = t
		}
	}
	a.tree.SetLibrary(a.lib)
}

// setItemTitle names a front-matter page, a document or a flat book's text
// from its own translation, unless a title is already known.
func (a *App) setItemTitle(n *library.Node, it api.Item) {
	if n == nil || !n.Leaf || n.Title != "" || a.learn.titles[n.Ref] != "" {
		return
	}
	front := len(n.Seg) == 2 && n.Seg[0] == '0' && n.Seg[1] >= 'a' && n.Seg[1] <= 'z'
	if !front && it.Document == nil && !(n.Parent != nil && n.Parent.Parent == nil) {
		return
	}
	line, _, _ := strings.Cut(strings.TrimSpace(it.Translation), "\n")
	t := api.SanitizeText(strings.Join(strings.Fields(line), " "))
	if t == "" || strings.ContainsAny(t, "*_#>") {
		return
	}
	a.learn.titles[n.Ref] = t
	a.applyLearned()
}
