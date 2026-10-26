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
	"vedabase-tui/internal/scrape"
)

const (
	siteURL     = "https://vedabase.cc"
	lettersURL  = siteURL + "/en/library/letters/"
	yearRefPfx  = "letters#year/"
	pageTimeout = 30 * time.Second
)

// Results of background scrapes. They are applied in Update, never in the Cmd.
type (
	titlesMsg struct {
		parent string
		titles map[string]string
	}
	yearsMsg    struct{ years []scrape.YearCount }
	yearPageMsg struct {
		year, page int
		entries    []scrape.LetterEntry
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

// scraped remembers what the site's HTML told us, so it survives the library
// being replaced by a background refresh.
type scraped struct {
	asked     map[string]bool   // containers whose titles were requested this session
	titles    map[string]string // node ref -> title
	years     []scrape.YearCount
	yearsAsk  bool
	yearPages map[int]*yearPages
}

func newScraped() scraped {
	return scraped{asked: map[string]bool{}, titles: map[string]string{}, yearPages: map[int]*yearPages{}}
}

// titlesCmd fetches the child titles of a container the first time it is
// needed. Only containers of containers (a book, a canto, a līlā) are listed
// with titles; a chapter's verses are not.
func (a *App) titlesCmd(n *library.Node) tea.Cmd {
	if n == nil || n.Leaf || strings.Contains(n.Ref, "#") || n.Book == "letters" || n.Book == "transcripts" {
		return nil
	}
	if a.scr.asked[n.Ref] {
		return nil
	}
	if _, ok := a.tocs[n.Book]; ok { // the API's contents already name every chapter
		return nil
	}
	hasContainer := false
	for _, c := range n.Children {
		if !c.Leaf {
			hasContainer = true
		}
	}
	if !hasContainer {
		return nil
	}
	a.scr.asked[n.Ref] = true
	ref, be := n.Ref, a.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pageTimeout)
		defer cancel()
		page, err := be.Page(ctx, siteURL+"/"+ref+"/")
		if err != nil {
			return titlesMsg{parent: ref}
		}
		return titlesMsg{parent: ref, titles: scrape.ChapterTitles(page, ref)}
	}
}

// ancestorTitlesCmd fetches titles for every container above n.
func (a *App) ancestorTitlesCmd(n *library.Node) tea.Cmd {
	var cmds []tea.Cmd
	for p := n; p != nil; p = p.Parent {
		if c := a.titlesCmd(p); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

func (a *App) onTitles(m titlesMsg) tea.Cmd {
	if len(m.titles) == 0 {
		return nil
	}
	if n := a.lib.Find(m.parent); n != nil {
		if _, ok := a.tocs[n.Book]; ok { // the contents arrived while this was on its way
			return nil
		}
	}
	for r, t := range m.titles {
		a.scr.titles[r] = t
	}
	a.applyScraped()
	return nil
}

// onExpandLazy loads the children of a Lazy node: the year list, or a year.
func (a *App) onExpandLazy(n *library.Node) tea.Cmd {
	if n == nil {
		return nil
	}
	switch {
	case n.Ref == "letters#year":
		if a.scr.yearsAsk {
			return nil
		}
		a.scr.yearsAsk = true
		be := a.be
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), pageTimeout)
			defer cancel()
			page, err := be.Page(ctx, lettersURL)
			if err != nil {
				return yearsMsg{}
			}
			return yearsMsg{years: scrape.LetterYears(page)}
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
	if len(m.years) == 0 {
		a.scr.yearsAsk = false // let a later expand retry
		return nil
	}
	a.scr.years = m.years
	a.applyScraped()
	return nil
}

// yearPageCmd fetches the next unloaded page of a year, if there is one and
// none is on its way.
func (a *App) yearPageCmd(y int) tea.Cmd {
	yp := a.scr.yearPages[y]
	if yp == nil {
		yp = &yearPages{more: true, titles: map[string]string{}}
		a.scr.yearPages[y] = yp
	}
	if !yp.more || yp.loading {
		return nil
	}
	yp.loading = true
	page, be := yp.loaded+1, a.be
	url := fmt.Sprintf("%s?year=%d", lettersURL, y)
	if page > 1 {
		url += fmt.Sprintf("&page=%d", page)
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pageTimeout)
		defer cancel()
		b, err := be.Page(ctx, url)
		if err != nil {
			return yearPageMsg{year: y, page: page}
		}
		es, more := scrape.LetterYearPage(b)
		return yearPageMsg{year: y, page: page, entries: es, more: more}
	}
}

func (a *App) onYearPage(m yearPageMsg) tea.Cmd {
	yp := a.scr.yearPages[m.year]
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
	a.applyScraped()
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

// applyScraped writes everything scraped so far onto the current library and
// refreshes the tree.
func (a *App) applyScraped() {
	if a.lib == nil {
		return
	}
	for _, yc := range a.scr.years {
		var refs []string
		var titles map[string]string
		if yp := a.scr.yearPages[yc.Year]; yp != nil {
			refs, titles = yp.refs, yp.titles
		}
		if g := a.lib.AddYearGroup(yc.Year, refs, titles); g != nil {
			g.Lazy = true
			g.Title = fmt.Sprintf("%d (%d)", yc.Year, yc.Count)
		}
	}
	for r, t := range a.scr.titles {
		if n := a.lib.Find(r); n != nil {
			n.Title = t
		}
	}
	a.tree.SetLibrary(a.lib)
}

// setItemTitle names a front-matter page, a document or a flat book's text
// from its own translation, unless a title is already known.
func (a *App) setItemTitle(n *library.Node, it api.Item) {
	if n == nil || !n.Leaf || n.Title != "" || a.scr.titles[n.Ref] != "" {
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
	a.scr.titles[n.Ref] = t
	a.applyScraped()
}
