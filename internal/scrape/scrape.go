// Package scrape reads the few things the API does not give: chapter titles
// from container pages and the letters-by-year listings, both from the
// site's HTML. Every function tolerates garbage and returns empty results.
//
// Letters listing URLs: https://vedabase.cc/en/library/letters/ (year facet in
// the sidebar), .../letters/?year=1967 for page 1 and
// .../letters/?year=1967&page=2 for page 2 (the "Next →" link of page 1).
package scrape

import (
	"bytes"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/ref"
)

type YearCount struct{ Year, Count int }

type LetterEntry struct{ Ref, Title, Date, Place string }

func parse(page []byte) *html.Node {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil
	}
	return doc
}

func walk(n *html.Node, f func(*html.Node)) {
	if n == nil {
		return
	}
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	return slices.Contains(strings.Fields(attr(n, "class")), class)
}

// text is the node's text with whitespace collapsed, sanitised.
func text(n *html.Node) string {
	var b strings.Builder
	walk(n, func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
			b.WriteByte(' ')
		}
	})
	return api.SanitizeText(strings.Join(strings.Fields(b.String()), " "))
}

// ChapterTitles maps each child ref listed on a container page to its link
// text, e.g. "bg/2" -> "Chapter 2: Contents of the Gītā Summarized". Only
// direct children of parentRef count.
func ChapterTitles(page []byte, parentRef string) map[string]string {
	out := map[string]string{}
	walk(parse(page), func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		r, ok := ref.FromPath(attr(n, "href"))
		if !ok {
			return
		}
		rest, found := strings.CutPrefix(r, parentRef+"/")
		if !found || rest == "" || strings.Contains(rest, "/") || strings.ContainsAny(rest, "#?") {
			return
		}
		if t := text(n); t != "" {
			out[r] = t
		}
	})
	return out
}

// LetterYears reads the year facet of the letters index, sorted by year.
func LetterYears(page []byte) []YearCount {
	var out []YearCount
	walk(parse(page), func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" || !hasClass(n, "vbl-facet-row") {
			return
		}
		u, err := url.Parse(attr(n, "href"))
		if err != nil || len(u.Query()) != 1 {
			return
		}
		y, err := strconv.Atoi(u.Query().Get("year"))
		if err != nil {
			return
		}
		var count string
		walk(n, func(c *html.Node) {
			if c.Type == html.ElementNode && hasClass(c, "vbl-facet-count") {
				count = text(c)
			}
		})
		count = strings.NewReplacer("(", "", ")", "", ",", "").Replace(count)
		cnt, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil {
			return
		}
		out = append(out, YearCount{y, cnt})
	})
	slices.SortFunc(out, func(a, b YearCount) int { return a.Year - b.Year })
	return out
}

// LetterYearPage reads one listing page: the letters on it and whether a
// "Next →" link follows.
func LetterYearPage(page []byte) (entries []LetterEntry, hasNext bool) {
	walk(parse(page), func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch {
		case n.Data == "a" && hasClass(n, "vbl-pgbtn"):
			if strings.HasPrefix(text(n), "Next") {
				hasNext = true
			}
		case n.Data == "a" && hasClass(n, "vbl-row-title"):
			r, ok := ref.FromPath(attr(n, "href"))
			if !ok || !strings.HasPrefix(r, "letters/") {
				return
			}
			e := LetterEntry{Ref: r, Title: text(n)}
			if p := n.Parent; p != nil {
				for c := p.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && hasClass(c, "vbl-row-meta") {
						date, place, _ := strings.Cut(text(c), " — ")
						e.Date, e.Place = strings.TrimSpace(date), strings.TrimSpace(place)
					}
				}
			}
			entries = append(entries, e)
		}
	})
	return entries, hasNext
}
