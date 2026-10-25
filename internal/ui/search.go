package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/render"
)

// searchResultsMsg carries one page of results (or the error) back to Search.
// gen and req identify the search and the page asked for, so replies to an
// abandoned search are dropped.
type searchResultsMsg struct {
	q, scope string
	page     api.SearchPage
	err      error
	gen, req int
}

// searchOpenMsg asks the App to open a result in the reader.
type searchOpenMsg struct{ ref string }

// validRef is what a result's ref must look like before it is shown or opened.
var validRef = regexp.MustCompile(`^[a-z0-9]+(/[a-z0-9_-]+){0,4}$`)

const (
	rowLines      = 3
	searchTimeout = 30 * time.Second
)

// Search is the full-text search overlay. It keeps its state while hidden so
// Esc and n/N can come back to the same results.
type Search struct {
	pal  render.Palette
	be   Backend
	hl   lipgloss.Style
	w, h int

	visible    bool
	focusInput bool
	input      textinput.Model
	scope      int // 0 all, 1 this book, 2 letters, 3 lectures
	bookCode   string
	bookTitle  string

	live      bool // a search has been run
	opened    bool // a result has been opened, so n continues from the cursor
	gen       int
	q, book   string
	results   []api.SearchResult
	page      int
	hasMore   bool
	loading   bool
	openAfter bool // open the first new result when the page in flight arrives
	cursor    int
	top       int
	errLine   string
}

func NewSearch(pal render.Palette) Search {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "search the books"
	st := in.Styles()
	st.Cursor.Blink = false // a steady cursor: no timers, nothing to wake the app
	in.SetStyles(st)
	return Search{pal: pal, input: in, hl: lipgloss.NewStyle().Foreground(pal.Gold).Bold(true)}
}

func (s *Search) SetSize(w, h int) { s.w, s.h = w, h; s.ensureVisible() }

func (s *Search) Live() bool { return s.live }

// Show opens the overlay. withInput focuses the query box; otherwise the list
// has the keys.
func (s *Search) Show(bookCode, bookTitle string, withInput bool) tea.Cmd {
	s.visible = true
	s.bookCode, s.bookTitle = bookCode, bookTitle
	if s.scope == 1 && bookCode == "" {
		s.scope = 0
	}
	s.focusInput = withInput || !s.live
	if s.focusInput {
		return s.input.Focus()
	}
	s.input.Blur()
	return nil
}

func (s *Search) scopeBook() string {
	switch s.scope {
	case 1:
		return s.bookCode
	case 2:
		return "letters"
	case 3:
		return "transcripts"
	}
	return ""
}

func (s *Search) cycleScope() {
	s.scope = (s.scope + 1) % 4
	if s.scope == 1 && s.bookCode == "" {
		s.scope = 2
	}
}

func (s *Search) fetch(page int) tea.Cmd {
	s.loading = true
	be, q, book, gen := s.be, s.q, s.book, s.gen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()
		p, err := be.Search(ctx, q, book, page)
		return searchResultsMsg{q: q, scope: book, page: p, err: err, gen: gen, req: page}
	}
}

// run starts a new search for the text in the box.
func (s *Search) run() tea.Cmd {
	q := strings.TrimSpace(s.input.Value())
	if q == "" {
		return nil
	}
	s.gen++
	s.q, s.book = q, s.scopeBook()
	s.live, s.opened, s.openAfter = true, false, false
	s.results, s.page, s.hasMore, s.cursor, s.top, s.errLine = nil, 0, false, 0, 0, ""
	s.focusInput = false
	s.input.Blur()
	return s.fetch(1)
}

func (s *Search) onResults(m searchResultsMsg) tea.Cmd {
	if m.gen != s.gen {
		return nil
	}
	s.loading = false
	if m.err != nil {
		s.openAfter = false
		var rl *api.RateLimitedError
		if errors.As(m.err, &rl) {
			s.errLine = fmt.Sprintf("Wait %ds before searching again", max(int(math.Ceil(rl.RetryAfter.Seconds())), 1))
			if m.req == 1 {
				s.focusInput = true
				return s.input.Focus()
			}
			return nil
		}
		msg := m.err.Error()
		return func() tea.Msg { return ToastMsg{Text: msg, Error: true} }
	}
	if m.req != s.page+1 {
		return nil
	}
	s.errLine = ""
	old := len(s.results)
	for _, r := range m.page.Results {
		if !validRef.MatchString(r.Ref) {
			continue
		}
		r.Translation = api.SanitizeText(r.Translation)
		r.SnippetPurport = api.SanitizeText(r.SnippetPurport)
		r.SnippetTranslation = api.SanitizeText(r.SnippetTranslation)
		r.BookTitle = api.SanitizeText(r.BookTitle)
		s.results = append(s.results, r)
	}
	s.page, s.hasMore = m.req, m.page.HasMore
	if s.openAfter {
		s.openAfter = false
		if len(s.results) > old {
			s.cursor = old
			return s.open()
		}
	}
	s.ensureVisible()
	return s.maybeMore()
}

// maybeMore loads the next page when the cursor is on the last row.
func (s *Search) maybeMore() tea.Cmd {
	if s.hasMore && !s.loading && len(s.results) > 0 && s.cursor >= len(s.results)-1 {
		return s.fetch(s.page + 1)
	}
	return nil
}

func (s *Search) move(to int) tea.Cmd {
	s.cursor = min(max(to, 0), max(len(s.results)-1, 0))
	s.ensureVisible()
	return s.maybeMore()
}

func (s *Search) open() tea.Cmd {
	if s.cursor < 0 || s.cursor >= len(s.results) {
		return nil
	}
	s.opened, s.visible = true, false
	r := s.results[s.cursor].Ref
	return func() tea.Msg { return searchOpenMsg{r} }
}

// Next opens the next (d=1) or previous (d=-1) result, loading the next page
// when the loaded ones run out. It works whether or not the overlay is shown.
func (s *Search) Next(d int) tea.Cmd {
	toast := func(t string) tea.Cmd { return func() tea.Msg { return ToastMsg{Text: t} } }
	if !s.live || (len(s.results) == 0 && !s.loading) {
		return toast("No search results")
	}
	if !s.opened {
		return s.open()
	}
	if d < 0 {
		if s.cursor == 0 {
			return toast("First search result")
		}
		s.cursor--
		s.ensureVisible()
		return s.open()
	}
	if s.cursor+1 < len(s.results) {
		s.cursor++
		s.ensureVisible()
		return s.open()
	}
	switch {
	case s.loading:
		s.openAfter = true
		return nil
	case s.hasMore:
		s.openAfter = true
		return s.fetch(s.page + 1)
	}
	return toast("No more search results")
}

// Update handles key presses (while visible) and result pages.
func (s *Search) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case searchResultsMsg:
		return s.onResults(m)
	case tea.KeyPressMsg:
		if !s.visible {
			return nil
		}
		return s.onKey(m)
	}
	return nil
}

func (s *Search) onKey(kp tea.KeyPressMsg) tea.Cmd {
	k := kp.String()
	switch k {
	case "esc":
		s.visible = false
		s.input.Blur()
		return nil
	case "tab":
		s.cycleScope()
		return nil
	case "enter":
		if s.focusInput {
			return s.run()
		}
		return s.open()
	case "down", "pgdown":
		return s.move(s.cursor + s.step(k))
	case "up", "pgup":
		return s.move(s.cursor - s.step(k))
	}
	if s.focusInput {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(kp)
		return cmd
	}
	switch k {
	case "j":
		return s.move(s.cursor + 1)
	case "k":
		return s.move(s.cursor - 1)
	case "G":
		return s.move(len(s.results) - 1)
	case "ctrl+d":
		return s.move(s.cursor + s.step("pgdown"))
	case "ctrl+u":
		return s.move(s.cursor - s.step("pgdown"))
	case "/", "i":
		s.focusInput = true
		return s.input.Focus()
	case "q":
		s.visible = false
	}
	return nil
}

func (s Search) step(k string) int {
	if strings.HasPrefix(k, "pg") {
		return max(s.visRows()-1, 1)
	}
	return 1
}

// Layout: border (2) + title, query, scope, message (4) + footer (1).
func (s Search) boxH() int { return max(min(s.h-2, 36), 9) }

func (s Search) visRows() int { return max((s.boxH()-7)/rowLines, 1) }

func (s *Search) ensureVisible() {
	v := s.visRows()
	if s.cursor < s.top {
		s.top = s.cursor
	}
	if s.cursor >= s.top+v {
		s.top = s.cursor - v + 1
	}
	s.top = max(s.top, 0)
}

var scopeNames = [...]string{"All", "This book", "Letters", "Lectures"}

func (s Search) scopeLine(inner int) string {
	dim := lipgloss.NewStyle().Foreground(s.pal.Dim)
	var parts []string
	for i, n := range scopeNames {
		if i == 1 && s.bookCode != "" && s.bookTitle != "" {
			n += " (" + s.bookTitle + ")"
		}
		if i == s.scope {
			parts = append(parts, lipgloss.NewStyle().Foreground(s.pal.Teal).Bold(true).Render(n))
		} else {
			parts = append(parts, dim.Render(n))
		}
	}
	return lipgloss.NewStyle().MaxWidth(inner).Render(strings.Join(parts, dim.Render(" · ")))
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func (s Search) row(i, inner int) []string {
	r := s.results[i]
	dim := lipgloss.NewStyle().Foreground(s.pal.Dim)
	mark := "  "
	if i == s.cursor {
		mark = lipgloss.NewStyle().Foreground(s.pal.Gold).Render("▸ ")
	}
	head := lipgloss.NewStyle().Foreground(s.pal.Bright).Bold(true).Render(ref.Display(r.Ref))
	if r.BookTitle != "" {
		head += dim.Render("  ·  " + r.BookTitle)
	}
	snip := r.SnippetPurport
	if strings.TrimSpace(snip) == "" {
		snip = r.SnippetTranslation
	}
	clip := lipgloss.NewStyle().MaxWidth(max(inner-2, 1))
	fg := lipgloss.NewStyle().Foreground(s.pal.Fg)
	return []string{
		clip.Render(mark + head),
		"  " + clip.Render(fg.Render(oneLine(r.Translation))),
		"  " + clip.Render(render.Marks(oneLine(snip), s.hl)),
	}
}

// View draws the overlay as a box; the caller centres it.
func (s Search) View() string {
	bw := max(min(s.w-4, 110), 24)
	inner := bw - 4
	dim := lipgloss.NewStyle().Foreground(s.pal.Dim)
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(s.pal.Gold).Bold(true).Render("Search"))
	s.input.SetWidth(max(inner-2, 1))
	lines = append(lines, dim.Render("/ ")+s.input.View(), s.scopeLine(inner))
	msg := ""
	switch {
	case s.errLine != "":
		msg = lipgloss.NewStyle().Foreground(s.pal.Rose).Render(s.errLine)
	case s.live && s.loading && len(s.results) == 0:
		msg = dim.Render("Searching…")
	case s.live && len(s.results) == 0:
		msg = dim.Render("No results")
	}
	lines = append(lines, msg)

	area := s.visRows() * rowLines
	var body []string
	for i := s.top; i < len(s.results) && len(body) < area; i++ {
		body = append(body, s.row(i, inner)...)
	}
	if s.loading && len(s.results) > 0 && len(body) < area {
		body = append(body, "  "+dim.Render("Loading more…"))
	}
	for len(body) < area {
		body = append(body, "")
	}
	lines = append(lines, body[:area]...)
	hint := "enter search · tab scope"
	if !s.focusInput {
		hint = "enter open · j/k move · tab scope · / edit query"
	}
	lines = append(lines, dim.Render(truncate(hint+" · esc close", inner)))

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(s.pal.Dim).
		Padding(0, 1).Width(bw).Render(strings.Join(lines, "\n"))
}
