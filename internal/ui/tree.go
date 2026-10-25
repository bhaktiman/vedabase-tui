package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/text/unicode/norm"

	"vedabase-tui/internal/library"
	"vedabase-tui/internal/render"
)

// OpenMsg asks the app to open a leaf.
type OpenMsg struct{ Node *library.Node }

// ExpandLazyMsg asks the app to load the children of a Lazy node; it should
// then hand the library back with SetLibrary.
type ExpandLazyMsg struct{ Node *library.Node }

// hintRef marks the row shown under an empty collections root.
const hintRef = "collections#hint"

const hintText = "Press s on a verse to start one"

type row struct {
	node  *library.Node
	depth int
}

// Tree is the contents tree: expandable, with a cursor, a filter and ★ marks.
type Tree struct {
	lib      *library.Library
	expanded map[string]bool // by Ref
	rows     []row           // visible rows, rebuilt when expansion or filter change
	cursor   int
	offset   int
	w, h     int
	filter   string
	shown    map[*library.Node]bool // filter mode: matches plus ancestors; nil otherwise
	starred  func(string) bool
	pal      render.Palette
	seq      keySeq
	focused  bool            // the tree pane has focus: the cursor row is bright, else dim
	opened   map[string]bool // Refs that were opened by default once; a later collapse sticks
}

func NewTree(lib *library.Library, starred func(ref string) bool, pal render.Palette) Tree {
	if starred == nil {
		starred = func(string) bool { return false }
	}
	t := Tree{lib: lib, expanded: map[string]bool{}, opened: map[string]bool{}, starred: starred, pal: pal}
	t.openNew()
	t.rebuild()
	return t
}

func (t *Tree) SetSize(w, h int) {
	t.w, t.h = w, h
	t.scroll()
}

func (t Tree) Selected() *library.Node {
	if t.cursor < 0 || t.cursor >= len(t.rows) {
		return nil
	}
	return t.rows[t.cursor].node
}

// SetLibrary swaps in a (possibly grown) library, keeping expansion and the
// cursor by Ref.
func (t *Tree) SetLibrary(lib *library.Library) {
	var sel *library.Node
	if n := t.Selected(); n != nil && !isHint(n) {
		sel = n
	}
	t.lib = lib
	t.openNew()
	t.matchFilter() // the kept filter refers to the old library's nodes
	t.rebuild()
	if sel != nil {
		// A collection item shares its ref with the book node, so match on
		// the collection it sits in as well.
		col, _ := library.InCollection(sel)
		first := -1
		for i, r := range t.rows {
			if r.node.Ref != sel.Ref {
				continue
			}
			if first < 0 {
				first = i
			}
			if c, _ := library.InCollection(r.node); c == col {
				first = i
				break
			}
		}
		if first >= 0 {
			t.cursor = first
		}
	}
	t.scroll()
}

// openNew expands the collections root and any collection it has not shown
// before, so they are visible at a glance; a collapse made later is kept.
func (t *Tree) openNew() {
	if t.lib == nil {
		return
	}
	for _, r := range t.lib.Roots {
		if r.Ref != library.CollectionsRef {
			continue
		}
		for _, n := range append([]*library.Node{r}, r.Children...) {
			if !t.opened[n.Ref] {
				t.opened[n.Ref] = true
				t.expanded[n.Ref] = true
			}
		}
	}
}

// CollapseAll collapses every node and clears the filter; the cursor goes to
// the top-level root that held the selected row.
func (t *Tree) CollapseAll() {
	t.SetFilter("")
	n := t.Selected()
	for n != nil && n.Parent != nil {
		n = n.Parent
	}
	t.expanded = map[string]bool{}
	t.rebuild()
	t.cursor = 0
	if i := t.indexOf(n); i >= 0 {
		t.cursor = i
	}
	t.scroll()
}

func isHint(n *library.Node) bool { return n != nil && n.Ref == hintRef }

// Reveal expands n's ancestors and puts the cursor on n.
func (t *Tree) Reveal(n *library.Node) {
	if n == nil {
		return
	}
	for p := n.Parent; p != nil; p = p.Parent {
		t.expanded[p.Ref] = true
	}
	if t.shown != nil && !t.shown[n] {
		t.filter, t.shown = "", nil
	}
	t.rebuild()
	if i := t.indexOf(n); i >= 0 {
		t.cursor = i
	}
	t.scroll()
}

// SetFilter shows only nodes whose label contains s (case and diacritics
// ignored), plus their ancestors, which are expanded. "" clears the filter.
func (t *Tree) SetFilter(s string) {
	prev := t.Selected()
	t.filter = s
	t.matchFilter()
	t.rebuild()
	t.cursor = 0
	if prev != nil {
		if i := t.indexOf(prev); i >= 0 {
			t.cursor = i
		}
	}
	t.scroll()
}

// matchFilter recomputes the set of nodes the current filter keeps.
func (t *Tree) matchFilter() {
	t.shown = nil
	if f := fold(t.filter); f != "" {
		t.shown = map[*library.Node]bool{}
		var walk func(n *library.Node) bool
		walk = func(n *library.Node) bool {
			keep := strings.Contains(fold(n.Label()), f)
			for _, c := range n.Children {
				if walk(c) {
					keep = true
				}
			}
			if keep {
				t.shown[n] = true
			}
			return keep
		}
		for _, r := range t.lib.Roots {
			walk(r)
		}
	}
}

func (t Tree) indexOf(n *library.Node) int {
	for i, r := range t.rows {
		if r.node == n {
			return i
		}
	}
	return -1
}

// rebuild recomputes the visible rows from the expanded set, walking only
// expanded branches.
func (t *Tree) rebuild() {
	t.rows = t.rows[:0]
	var walk func(n *library.Node, depth int)
	walk = func(n *library.Node, depth int) {
		if t.shown != nil && !t.shown[n] {
			return
		}
		t.rows = append(t.rows, row{n, depth})
		if t.shown != nil || t.expanded[n.Ref] {
			for _, c := range n.Children {
				walk(c, depth+1)
			}
			if t.shown == nil && n.Ref == library.CollectionsRef && len(n.Children) == 0 {
				t.rows = append(t.rows, row{&library.Node{Ref: hintRef, Title: hintText, Parent: n}, 0})
			}
		}
	}
	if t.lib != nil {
		for _, r := range t.lib.Roots {
			walk(r, 0)
		}
	}
	if t.cursor >= len(t.rows) {
		t.cursor = len(t.rows) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

func (t *Tree) scroll() {
	if t.h <= 0 {
		t.offset = 0
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+t.h {
		t.offset = t.cursor - t.h + 1
	}
	if m := len(t.rows) - t.h; t.offset > m {
		t.offset = max(m, 0)
	}
}

func (t *Tree) move(to int) {
	t.cursor = min(max(to, 0), len(t.rows)-1)
	t.scroll()
}

func (t Tree) Update(msg tea.Msg) (Tree, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return t, nil
	}
	k := t.seq.Resolve(kp)
	n := t.Selected()
	if n == nil {
		return t, nil
	}
	half := max(t.h/2, 1)
	switch k {
	case "j", "down":
		t.move(t.cursor + 1)
	case "k", "up":
		t.move(t.cursor - 1)
	case "top":
		t.move(0)
	case "bottom":
		t.move(len(t.rows) - 1)
	case "halfdown":
		t.move(t.cursor + half)
	case "halfup":
		t.move(t.cursor - half)
	case "l", "right", "enter":
		if isHint(n) {
			return t, nil
		}
		if n.Leaf {
			if k == "right" {
				return t, nil
			}
			return t, func() tea.Msg { return OpenMsg{n} }
		}
		return t.expand(n)
	case "h", "left":
		t.collapse(n)
	}
	return t, nil
}

// Click handles a left click on visible row idx. A click on the ▸/▾ marker, or
// on an already selected container, toggles it; a leaf opens, as Enter does.
func (t Tree) Click(idx int, onMarker bool) (Tree, tea.Cmd) {
	if idx < 0 || idx >= len(t.rows) {
		return t, nil
	}
	n := t.rows[idx].node
	was := t.cursor == idx
	t.move(idx)
	switch {
	case isHint(n):
		return t, nil
	case n.Leaf:
		return t, func() tea.Msg { return OpenMsg{n} }
	case !was && !onMarker:
		return t, nil
	case t.shown != nil: // filter mode keeps every branch open
		return t, nil
	case t.expanded[n.Ref]:
		t.collapse(n)
		return t, nil
	}
	return t.expand(n)
}

// Scroll moves the view by d rows; the cursor stays unless it would leave it.
func (t *Tree) Scroll(d int) {
	t.offset = min(max(t.offset+d, 0), max(len(t.rows)-t.h, 0))
	t.cursor = min(max(t.cursor, t.offset), max(t.offset+t.h-1, 0))
	t.cursor = min(t.cursor, max(len(t.rows)-1, 0))
}

func (t Tree) expand(n *library.Node) (Tree, tea.Cmd) {
	if t.expanded[n.Ref] && len(n.Children) > 0 {
		if t.shown == nil || t.cursor+1 < len(t.rows) && t.rows[t.cursor+1].node.Parent == n {
			t.move(t.cursor + 1) // already open: step into it
		}
		return t, nil
	}
	t.expanded[n.Ref] = true
	t.rebuild()
	t.scroll()
	if n.Lazy && len(n.Children) == 0 {
		return t, func() tea.Msg { return ExpandLazyMsg{n} }
	}
	return t, nil
}

func (t *Tree) collapse(n *library.Node) {
	// In filter mode every shown branch is open, so h only goes up.
	if t.shown == nil && !n.Leaf && t.expanded[n.Ref] {
		delete(t.expanded, n.Ref)
		t.rebuild()
		t.scroll()
		return
	}
	if n.Parent != nil {
		if i := t.indexOf(n.Parent); i >= 0 {
			t.move(i)
		}
	}
}

func (t Tree) View() string {
	if t.w <= 0 || t.h <= 0 {
		return ""
	}
	normal := lipgloss.NewStyle().Foreground(t.pal.Fg)
	dim := lipgloss.NewStyle().Foreground(t.pal.Dim)
	gold := lipgloss.NewStyle().Foreground(t.pal.Gold)
	cur := lipgloss.NewStyle().Foreground(t.pal.Accent).Reverse(true)

	lines := make([]string, t.h)
	for i := range lines {
		idx := t.offset + i
		if idx >= len(t.rows) {
			continue
		}
		r := t.rows[idx]
		marker := "  "
		if isHint(r.node) {
			marker = "" // the hint is long; it gets the whole width
		} else if !r.node.Leaf {
			marker = "▸ "
			if t.shown != nil || t.expanded[r.node.Ref] {
				marker = "▾ "
			}
		}
		star := ""
		if r.node.Leaf && !inCollections(r.node) && t.starred(r.node.Ref) {
			star = "★ "
		}
		indent := strings.Repeat("  ", r.depth)
		label := r.node.Label()
		count := ""
		if p := r.node.Parent; p != nil && p.Ref == library.CollectionsRef {
			count = fmt.Sprintf("  %d", len(r.node.Children))
		}

		if idx == t.cursor && !t.focused {
			// A quiet cursor: dim text and a › in the left margin.
			s := truncate("›"+indent+marker+star+label+count, t.w)
			if r.depth > 0 {
				s = truncate(indent[:len(indent)-1]+"›"+marker+star+label+count, t.w)
			}
			lines[i] = dim.Render(s)
			continue
		}
		if idx == t.cursor {
			s := truncate(indent+marker+star+label+count, t.w)
			lines[i] = cur.Render(s + strings.Repeat(" ", t.w-lipgloss.Width(s)))
			continue
		}
		// Style the pieces; each is truncated to what is left of the width.
		left := t.w
		var b strings.Builder
		put := func(s string, st lipgloss.Style) {
			s = truncate(s, left)
			left -= lipgloss.Width(s)
			if s != "" {
				b.WriteString(st.Render(s))
			}
		}
		put(indent, normal)
		put(marker, dim)
		put(star, gold)
		if isHint(r.node) {
			put(label, dim)
		} else {
			put(label, normal)
		}
		put(count, dim)
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}

// inCollections reports whether n is an item or collection under the
// collections root.
func inCollections(n *library.Node) bool {
	_, ok := library.InCollection(n)
	return ok
}

// truncate cuts s to at most w cells, ending in "…" when it had to cut.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	b.WriteString("…")
	return b.String()
}

// fold lowercases s and strips diacritics.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
