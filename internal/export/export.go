// Package export renders items and collections as Markdown files.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/store"
)

// Meta is what an item does not carry itself. An empty Chapter is omitted.
type Meta struct{ BookTitle, Chapter string }

// Entry is one member of a collection; a nil Item means it could not be fetched.
type Entry struct {
	Ref  string
	Item *api.Item
	Meta Meta
}

const dateLayout = "2006-01-02"

func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.NewReplacer("\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(s)
	return `"` + s + `"`
}

func kv(b *strings.Builder, k, v string) { fmt.Fprintf(b, "%s: %s\n", k, quote(v)) }

// label is the heading of an item: its title for documents, else its short ref.
func label(it api.Item, r string) string {
	if it.Document != nil && strings.TrimSpace(it.Translation) != "" {
		return strings.TrimSpace(it.Translation)
	}
	return ref.Display(r)
}

// parts returns the visible non-empty parts joined by blank lines. For a
// document the translation is its title and already serves as the heading.
func parts(it api.Item, p api.Parts) string {
	var out []string
	add := func(on bool, s string) {
		if s = strings.TrimSpace(s); on && s != "" {
			out = append(out, s)
		}
	}
	add(p.Devanagari, it.Devanagari)
	add(p.Verse, it.VerseText)
	add(p.Synonyms, it.Synonyms)
	add(p.Translation && it.Document == nil, it.Translation)
	add(p.Purport, it.Purport)
	return strings.Join(out, "\n\n")
}

// ItemMarkdown renders one item with front matter.
func ItemMarkdown(it api.Item, m Meta, p api.Parts, today time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	kv(&b, "ref", it.Ref)
	kv(&b, "book", it.Book)
	kv(&b, "book_title", m.BookTitle)
	if m.Chapter != "" {
		kv(&b, "chapter", m.Chapter)
	}
	kv(&b, "url", it.URL)
	kv(&b, "exported", today.Format(dateLayout))
	if d := it.Document; d != nil {
		kv(&b, "date", d.Date)
		if d.Year != 0 {
			b.WriteString("year: " + strconv.Itoa(d.Year) + "\n")
		}
		kv(&b, "location", d.Location)
	}
	b.WriteString("---\n\n# " + label(it, it.Ref) + "\n")
	if s := parts(it, p); s != "" {
		b.WriteString("\n" + s + "\n")
	}
	return b.String()
}

// CollectionMarkdown renders a whole collection as a single file.
func CollectionMarkdown(name string, entries []Entry, p api.Parts, today time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	kv(&b, "collection", name)
	kv(&b, "exported", today.Format(dateLayout))
	b.WriteString("count: " + strconv.Itoa(len(entries)) + "\n")
	b.WriteString("---\n\n# " + name + "\n")
	for _, e := range entries {
		if e.Item == nil {
			b.WriteString("\n## " + ref.Display(e.Ref) + "\n\n(unavailable: " + e.Ref + ")\n")
			continue
		}
		it := *e.Item
		b.WriteString("\n## " + label(it, e.Ref) + "\n\n")
		b.WriteString(e.Meta.BookTitle + " · [vedabase.cc](" + it.URL + ")\n")
		if s := parts(it, p); s != "" {
			b.WriteString("\n" + s + "\n")
		}
	}
	return b.String()
}

// ItemFilename is the export file name for a ref, e.g. "bg-2-13.md".
func ItemFilename(r string) string { return ref.Filename(r) + ".md" }

// CollectionFilename is the export file name for a collection.
func CollectionFilename(name string) string { return "collection-" + ref.Slug(name) + ".md" }

// Write creates dir if needed and atomically writes content, reporting
// whether a file of that name was already there.
func Write(dir, filename, content string) (path string, existed bool, err error) {
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	path = filepath.Join(dir, filename)
	if _, statErr := os.Stat(path); statErr == nil {
		existed = true
	}
	if err = store.WriteFileAtomic(path, []byte(content)); err != nil {
		return "", existed, err
	}
	return path, existed, nil
}
