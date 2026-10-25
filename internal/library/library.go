// Package library builds the contents tree of the vedabase from the site's
// sitemap and defines the reading order over it.
package library

import (
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/ref"
)

const (
	lettersCode     = "letters"
	transcriptsCode = "transcripts"
	lettersYearRef  = "letters#year"
	lettersToRef    = "letters#to"

	// CollectionsRef is the ref of the virtual root above all books; its
	// children are the user's collections ("collections#/<name>").
	CollectionsRef  = "collections#"
	collectionsCode = "collections"
)

// Node is one entry of the contents tree.
type Node struct {
	Ref      string // "bg", "bg/2", "bg/2/13"; virtual: "letters#year", "letters#to", "letters#to/abhirama", "transcripts#1966"
	Seg      string // last segment ("13", "adi", "0a"); for virtual nodes the group key
	Title    string // set by scraping / fetched items; empty until known
	Leaf     bool   // readable item
	Lazy     bool   // children loaded on demand (letters by year)
	Children []*Node
	Parent   *Node
	Book     string // book code of the root this node is under

	flat bool // the book's texts hang directly off the root (api.Book.Type "flat")
}

// Library is the parsed tree plus the indexes used to navigate it.
type Library struct {
	Roots []*Node

	index  map[string]*Node   // primary node per ref
	roots  map[string]*Node   // root per book code
	flat   map[string][]*Node // leaves in reading order, per book root
	flatAt map[*Node]int      // position of a leaf in its book's flat list
}

// Parse reads a sitemap and builds the tree. books gives the order and titles
// of the roots; only books that appear in the sitemap get a root. Books in the
// sitemap but not in books follow the known ones, labelled by their code.
func Parse(sitemap io.Reader, books []api.Book) (*Library, error) {
	l := &Library{
		index: map[string]*Node{},
		roots: map[string]*Node{},
	}
	cols := &Node{Ref: CollectionsRef, Seg: collectionsCode, Title: "★ Collections", Book: collectionsCode}
	l.index[cols.Ref] = cols
	seen := map[string]bool{}
	var order []string // codes in first-seen order, for unknown books

	dec := xml.NewDecoder(sitemap)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse sitemap: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "loc" {
			continue
		}
		var loc string
		if err := dec.DecodeElement(&loc, &se); err != nil {
			return nil, fmt.Errorf("parse sitemap: %w", err)
		}
		r, ok := ref.FromPath(strings.TrimSpace(loc))
		if !ok || r == "library" {
			continue
		}
		code := r
		if i := strings.IndexByte(r, '/'); i >= 0 {
			code = r[:i]
		}
		if !seen[code] {
			seen[code] = true
			order = append(order, code)
		}
		l.add(r, code)
	}

	titles := map[string]string{}
	for _, b := range books {
		titles[b.Code] = b.Title
	}
	for _, b := range books {
		if r := l.roots[b.Code]; r != nil {
			r.flat = b.Type == "flat"
		}
	}
	var codes []string
	for _, b := range books {
		if seen[b.Code] && b.Code != lettersCode && b.Code != transcriptsCode && !slices.Contains(codes, b.Code) {
			codes = append(codes, b.Code)
		}
	}
	for _, c := range order {
		if _, known := titles[c]; !known && c != lettersCode && c != transcriptsCode {
			codes = append(codes, c)
		}
	}
	for _, c := range []string{lettersCode, transcriptsCode} {
		if seen[c] {
			codes = append(codes, c)
		}
	}
	for _, c := range codes {
		root := l.roots[c]
		if root == nil {
			continue
		}
		root.Title = titles[c] // empty for unknown books: Label falls back to the code
		l.Roots = append(l.Roots, root)
	}
	l.Roots = append([]*Node{cols}, l.Roots...)

	for _, root := range l.Roots {
		switch root.Ref {
		case lettersCode:
			l.groupLetters(root)
		case transcriptsCode:
			l.groupTranscripts(root)
		default:
			finish(root)
		}
	}
	l.flatten()
	return l, nil
}

// add inserts ref r (and any missing ancestors) into the book tree.
func (l *Library) add(r, code string) {
	segs := strings.Split(r, "/")
	if (code == lettersCode || code == transcriptsCode) && len(segs) > 2 {
		return
	}
	var parent *Node
	for i := range segs {
		cur := strings.Join(segs[:i+1], "/")
		n := l.index[cur]
		if n == nil {
			n = &Node{Ref: cur, Seg: segs[i], Parent: parent, Book: code}
			l.index[cur] = n
			if parent == nil {
				l.roots[code] = n
			} else {
				parent.Children = append(parent.Children, n)
			}
		}
		parent = n
	}
}

// finish sorts the children of n's subtree naturally and marks leaves.
func finish(n *Node) {
	slices.SortFunc(n.Children, func(a, b *Node) int { return ref.CompareSegment(a.Seg, b.Seg) })
	for _, c := range n.Children {
		finish(c)
	}
	n.Leaf = n.Parent != nil && len(n.Children) == 0
}

// groupLetters replaces the flat slug list under the letters root with the
// "by year" (lazy, filled later) and "by recipient" groups.
func (l *Library) groupLetters(root *Node) {
	slugs := root.Children
	root.Children = nil
	for _, s := range slugs {
		delete(l.index, s.Ref)
	}
	year := &Node{Ref: lettersYearRef, Seg: "year", Lazy: true, Parent: root, Book: lettersCode}
	to := &Node{Ref: lettersToRef, Seg: "to", Parent: root, Book: lettersCode}
	root.Children = []*Node{year, to}
	l.index[year.Ref], l.index[to.Ref] = year, to

	groups := map[string]*Node{}
	for _, s := range slugs {
		key := ref.Recipient(s.Seg)
		if key == "" {
			key = "other"
		}
		g := groups[key]
		if g == nil {
			g = &Node{Ref: lettersToRef + "/" + key, Seg: key, Parent: to, Book: lettersCode}
			groups[key] = g
			to.Children = append(to.Children, g)
			l.index[g.Ref] = g
		}
		s.Parent, s.Leaf = g, true
		g.Children = append(g.Children, s)
		l.index[s.Ref] = s
	}
	slices.SortFunc(to.Children, func(a, b *Node) int { return strings.Compare(a.Seg, b.Seg) })
	for _, g := range to.Children {
		slices.SortFunc(g.Children, func(a, b *Node) int { return ref.CompareSegment(a.Seg, b.Seg) })
	}
}

// groupTranscripts files each transcript under its year, ordered by date then slug.
func (l *Library) groupTranscripts(root *Node) {
	slugs := root.Children
	root.Children = nil
	for _, s := range slugs {
		delete(l.index, s.Ref)
	}
	groups := map[string]*Node{}
	for _, s := range slugs {
		key := "undated"
		if d, ok := ref.TranscriptDate(s.Seg); ok {
			key = strconv.Itoa(d.Year())
		}
		g := groups[key]
		if g == nil {
			g = &Node{Ref: transcriptsCode + "#" + key, Seg: key, Parent: root, Book: transcriptsCode}
			groups[key] = g
			root.Children = append(root.Children, g)
			l.index[g.Ref] = g
		}
		s.Parent, s.Leaf = g, true
		g.Children = append(g.Children, s)
		l.index[s.Ref] = s
	}
	slices.SortFunc(root.Children, func(a, b *Node) int { return strings.Compare(a.Seg, b.Seg) })
	for _, g := range root.Children {
		slices.SortFunc(g.Children, func(a, b *Node) int {
			da, _ := ref.TranscriptDate(a.Seg)
			db, _ := ref.TranscriptDate(b.Seg)
			if c := da.Compare(db); c != 0 {
				return c
			}
			return strings.Compare(a.Seg, b.Seg)
		})
	}
}

// flatten caches the leaves of every ordinary book in reading order.
func (l *Library) flatten() {
	l.flat = map[string][]*Node{}
	l.flatAt = map[*Node]int{}
	for _, root := range l.Roots {
		if root.Ref == lettersCode || root.Ref == transcriptsCode || root.Ref == CollectionsRef {
			continue
		}
		var leaves []*Node
		var walk func(n *Node)
		walk = func(n *Node) {
			if n.Leaf {
				l.flatAt[n] = len(leaves)
				leaves = append(leaves, n)
			}
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(root)
		l.flat[root.Ref] = leaves
	}
}

// Find returns the primary node for a ref: the book tree, the By-recipient
// node for letters, the year group for transcripts.
func (l *Library) Find(r string) *Node { return l.index[r] }

// AddYearGroup creates (or returns) the node letters#year/<year>, keeps the
// year groups in order, and adds a leaf for each ref not already in the group.
// titles are applied to the new leaves and to the By-recipient nodes with the
// same ref. It returns nil if the library has no letters.
func (l *Library) AddYearGroup(year int, refs []string, titles map[string]string) *Node {
	parent := l.index[lettersYearRef]
	if parent == nil {
		return nil
	}
	key := strconv.Itoa(year)
	g := l.index[lettersYearRef+"/"+key]
	if g == nil {
		g = &Node{Ref: lettersYearRef + "/" + key, Seg: key, Parent: parent, Book: lettersCode}
		l.index[g.Ref] = g
		parent.Children = append(parent.Children, g)
		slices.SortFunc(parent.Children, func(a, b *Node) int { return ref.CompareSegment(a.Seg, b.Seg) })
	}
	have := map[string]bool{}
	for _, c := range g.Children {
		have[c.Ref] = true
	}
	for _, r := range refs {
		if !have[r] {
			have[r] = true
			g.Children = append(g.Children, &Node{
				Ref: r, Seg: r[strings.LastIndexByte(r, '/')+1:], Leaf: true,
				Parent: g, Book: lettersCode, Title: titles[r],
			})
		}
		if t, ok := titles[r]; ok {
			if n := l.index[r]; n != nil {
				n.Title = t
			}
		}
	}
	for _, c := range g.Children {
		if t, ok := titles[c.Ref]; ok {
			c.Title = t
		}
	}
	return g
}

// CollectionItem is one entry of a collection as the sidebar shows it.
type CollectionItem struct{ Ref, Book, Label string }

// CollectionView is a collection as the sidebar shows it. The library takes
// plain values so it need not know the collections store.
type CollectionView struct {
	Name  string
	Items []CollectionItem
}

// SetCollections replaces the children of the collections root. Item nodes are
// separate from the book tree's: Find still returns the book node for a ref.
func (l *Library) SetCollections(cols []CollectionView) {
	root := l.index[CollectionsRef]
	if root == nil {
		return
	}
	root.Children = nil
	for _, c := range cols {
		cn := &Node{Ref: CollectionsRef + "/" + c.Name, Seg: c.Name, Title: c.Name, Parent: root, Book: collectionsCode}
		for _, it := range c.Items {
			cn.Children = append(cn.Children, &Node{
				Ref: it.Ref, Seg: it.Ref[strings.LastIndexByte(it.Ref, '/')+1:], Title: it.Label,
				Leaf: true, Parent: cn, Book: it.Book,
			})
		}
		root.Children = append(root.Children, cn)
	}
}

// CollectionItem returns the node of ref inside the named collection, or nil.
func (l *Library) CollectionItem(name, ref string) *Node {
	root := l.index[CollectionsRef]
	if root == nil {
		return nil
	}
	for _, c := range root.Children {
		if c.Seg == name {
			for _, it := range c.Children {
				if it.Ref == ref {
					return it
				}
			}
		}
	}
	return nil
}

// InCollection reports whether n is an item or a collection under the
// collections root, and the collection's name.
func InCollection(n *Node) (name string, ok bool) {
	for ; n != nil; n = n.Parent {
		if n.Parent != nil && n.Parent.Ref == CollectionsRef {
			return n.Seg, true
		}
	}
	return "", false
}
