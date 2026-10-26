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
	md, _ := displayMarkdown(it, p, false)
	return md
}

// DisplayLinked is DisplayMarkdown with the item's references marked by
// sentinels (see links.go). refs lists the target of each marked span in order
// of appearance; pass both to Renderer.Render.
func DisplayLinked(it api.Item, p api.Parts) (md string, refs []string) {
	return displayMarkdown(it, p, true)
}

func displayMarkdown(it api.Item, p api.Parts, link bool) (string, []string) {
	var refs []string
	// a stray sentinel in any field would throw off every link after it
	it.Devanagari = sentinelStripper.Replace(it.Devanagari)
	it.VerseText = sentinelStripper.Replace(it.VerseText)
	it.Synonyms = sentinelStripper.Replace(it.Synonyms)
	it.Translation = sentinelStripper.Replace(it.Translation)
	it.Purport = sentinelStripper.Replace(it.Purport)
	if it.Document != nil {
		d := *it.Document // the caller's copy stays as it is
		d.Date = sentinelStripper.Replace(d.Date)
		d.Location = sentinelStripper.Replace(d.Location)
		it.Document = &d
	}
	// shown fields are marked exactly when they are placed in the output, so
	// refs follows the order of appearance and hidden parts contribute none.
	field := func(name, text string) string {
		if link {
			text = markRefs(text, name, it.References, &refs)
		}
		return inlineLinks(text)
	}
	it.Synonyms = inlineLinks(it.Synonyms)
	it.VerseText = inlineLinks(it.VerseText)
	var sections []string
	prose := strings.TrimSpace(it.Devanagari+it.VerseText+it.Synonyms) == ""
	if it.Document != nil || prose {
		sections = append(sections, "## "+field("translation", it.Translation))
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
			sections = append(sections, quoteBreaks(field("purport", it.Purport)))
		}
		return strings.Join(sections, "\n\n"), refs
	}
	sections = append(sections, "## "+ref.Display(it.Ref))
	add := func(show bool, heading, text string) {
		if show && strings.TrimSpace(text) != "" {
			sections = append(sections, "#### "+heading, text)
		}
	}
	addField := func(show bool, heading, name, text string, br func(string) string) {
		if show && strings.TrimSpace(text) != "" {
			sections = append(sections, "#### "+heading, br(field(name, text)))
		}
	}
	id := func(s string) string { return s }
	add(p.Devanagari, "Devanagari", lineBreaks(it.Devanagari))
	add(p.Verse, "Verse", lineBreaks(it.VerseText))
	add(p.Synonyms, "Word for word", it.Synonyms)
	addField(p.Translation, "Translation", "translation", it.Translation, id)
	addField(p.Purport, "Purport", "purport", it.Purport, quoteBreaks)
	return strings.Join(sections, "\n\n"), refs
}

var wikiLink = regexp.MustCompile(`\[\[([^\[\]|]*)(?:\|([^\[\]]*))?\]\]`)

// inlineLinks shows the site's [[ref|label]] links as their label, or the ref
// when there is none.
func inlineLinks(s string) string {
	if !strings.Contains(s, "[[") {
		return s
	}
	return wikiLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := wikiLink.FindStringSubmatch(m)
		if sub[2] != "" {
			return sub[2]
		}
		return sub[1]
	})
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
	lang  string
	width int
	parts api.Parts
}

type cacheEntry struct {
	key   cacheKey
	out   string
	links []Link
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

// Render returns md styled for the given width, cached by {ref, lang, width, parts}.
// md may carry the sentinels of DisplayLinked, with refs as its targets: they
// are stripped, the spans underlined, and their positions returned. The links
// are shared with the cache and must not be modified.
func (r *Renderer) Render(ref, lang, md string, refs []string, width int, p api.Parts) (string, []Link, error) {
	key := cacheKey{ref, lang, width, p}
	r.mu.Lock()
	if e, ok := r.entries[key]; ok {
		r.lru.MoveToFront(e)
		ce := e.Value.(*cacheEntry)
		r.mu.Unlock()
		return ce.out, ce.links, nil
	}
	pal := r.p
	r.mu.Unlock()

	tr, err := glamour.NewTermRenderer(glamour.WithStyles(GlamourStyle(pal)), glamour.WithWordWrap(width), glamour.WithPreservedNewLines())
	if err != nil {
		return "", nil, err
	}
	out, err := tr.Render(md)
	if err != nil {
		return "", nil, err
	}
	var links []Link
	out, links = applyLinks(out, refs, pal.Accent)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.p.Name != pal.Name { // the palette changed while rendering: drop this result
		return out, links, nil
	}
	r.renders++
	if e, ok := r.entries[key]; ok { // a concurrent render won; keep one entry
		ce := e.Value.(*cacheEntry)
		ce.out, ce.links = out, links
		r.lru.MoveToFront(e)
		return out, links, nil
	}
	r.entries[key] = r.lru.PushFront(&cacheEntry{key, out, links})
	if r.lru.Len() > cacheSize {
		old := r.lru.Back()
		r.lru.Remove(old)
		delete(r.entries, old.Value.(*cacheEntry).key)
	}
	return out, links, nil
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
