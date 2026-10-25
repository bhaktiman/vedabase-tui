package render

import (
	"container/list"
	"html"
	"regexp"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/ref"
)

// DisplayMarkdown lays an item out as Markdown, skipping hidden or empty parts.
// Documents and prose chapters (no verse parts) show title, date and body
// instead; the part toggles do not apply to them.
func DisplayMarkdown(it api.Item, p api.Parts) string {
	var sections []string
	prose := strings.TrimSpace(it.Devanagari+it.VerseText+it.Synonyms) == ""
	if it.Document != nil || prose {
		sections = append(sections, "## "+it.Translation)
		var meta []string
		if it.Document != nil {
			for _, s := range []string{it.Document.Date, it.Document.Location} {
				if s != "" {
					meta = append(meta, s)
				}
			}
		}
		if len(meta) > 0 {
			sections = append(sections, strings.Join(meta, " — "))
		}
		if it.Purport != "" {
			sections = append(sections, quoteBreaks(it.Purport))
		}
		return strings.Join(sections, "\n\n")
	}
	sections = append(sections, "## "+ref.Display(it.Ref))
	add := func(show bool, heading, text string) {
		if show && strings.TrimSpace(text) != "" {
			sections = append(sections, "#### "+heading, text)
		}
	}
	add(p.Devanagari, "Devanagari", lineBreaks(it.Devanagari))
	add(p.Verse, "Verse", lineBreaks(it.VerseText))
	add(p.Synonyms, "Word for word", it.Synonyms)
	add(p.Translation, "Translation", it.Translation)
	add(p.Purport, "Purport", quoteBreaks(it.Purport))
	return strings.Join(sections, "\n\n")
}

// lineBreaks ends every non-blank line that is followed by another non-blank
// line with a Markdown hard break, so verse lines stay on separate rows.
func lineBreaks(s string) string { return breaks(s, func(string) bool { return true }) }

// quoteBreaks does the same for consecutive "> " quote lines (verses quoted in
// purports and lectures).
func quoteBreaks(s string) string {
	return breaks(s, func(l string) bool { return strings.HasPrefix(l, ">") })
}

func breaks(s string, want func(string) bool) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := 0; i+1 < len(lines); i++ {
		if want(lines[i]) && strings.TrimSpace(lines[i]) != "" && strings.TrimSpace(lines[i+1]) != "" &&
			want(lines[i+1]) && !strings.HasSuffix(lines[i], "\\") {
			lines[i] += "\\"
		}
	}
	return strings.Join(lines, "\n")
}

type cacheKey struct {
	ref   string
	width int
	parts api.Parts
}

type cacheEntry struct {
	key cacheKey
	out string
}

const cacheSize = 32

// Renderer renders Markdown with the palette's Glamour style and remembers the
// last few results. It is safe for concurrent use.
type Renderer struct {
	p       Palette
	mu      sync.Mutex
	lru     *list.List // front = most recent
	entries map[cacheKey]*list.Element
	renders int // number of real renders, for tests
}

func NewRenderer(p Palette) *Renderer {
	return &Renderer{p: p, lru: list.New(), entries: map[cacheKey]*list.Element{}}
}

// SetPalette switches the style and forgets everything rendered with the old one.
func (r *Renderer) SetPalette(p Palette) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.p = p
	r.lru.Init()
	r.entries = map[cacheKey]*list.Element{}
}

// Render returns md styled for the given width, cached by {ref, width, parts}.
func (r *Renderer) Render(ref, md string, width int, p api.Parts) (string, error) {
	key := cacheKey{ref, width, p}
	r.mu.Lock()
	if e, ok := r.entries[key]; ok {
		r.lru.MoveToFront(e)
		out := e.Value.(*cacheEntry).out
		r.mu.Unlock()
		return out, nil
	}
	pal := r.p
	r.mu.Unlock()

	tr, err := glamour.NewTermRenderer(glamour.WithStyles(GlamourStyle(pal)), glamour.WithWordWrap(width), glamour.WithPreservedNewLines())
	if err != nil {
		return "", err
	}
	out, err := tr.Render(md)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.p.Name != pal.Name { // the palette changed while rendering: drop this result
		return out, nil
	}
	r.renders++
	if e, ok := r.entries[key]; ok { // a concurrent render won; keep one entry
		e.Value.(*cacheEntry).out = out
		r.lru.MoveToFront(e)
		return out, nil
	}
	r.entries[key] = r.lru.PushFront(&cacheEntry{key, out})
	if r.lru.Len() > cacheSize {
		old := r.lru.Back()
		r.lru.Remove(old)
		delete(r.entries, old.Value.(*cacheEntry).key)
	}
	return out, nil
}

var (
	markRe = regexp.MustCompile(`(?s)<mark>(.*?)</mark>`)
	tagRe  = regexp.MustCompile(`<[^>]*>`)
)

// Marks styles <mark> spans of a search snippet with hl, strips other tags and
// unescapes HTML entities.
func Marks(snippet string, hl lipgloss.Style) string {
	var b strings.Builder
	last := 0
	// Sanitise after unescaping: entities like &#27; decode into live controls.
	clean := func(s string) string { return api.SanitizeText(html.UnescapeString(tagRe.ReplaceAllString(s, ""))) }
	emit := func(s string) { b.WriteString(clean(s)) }
	for _, m := range markRe.FindAllStringSubmatchIndex(snippet, -1) {
		emit(snippet[last:m[0]])
		b.WriteString(hl.Render(clean(snippet[m[2]:m[3]])))
		last = m[1]
	}
	emit(snippet[last:])
	return b.String()
}
