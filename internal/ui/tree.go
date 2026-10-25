package ui

import (
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
}

func NewTree(lib *library.Library, starred func(ref string) bool, pal render.Palette) Tree {
	if starred == nil {
		starred = func(string) bool { return false }
	}
	t := Tree{lib: lib, expanded: map[string]bool{}, starred: starred, pal: pal}
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
	ref := ""
	if n := t.Selected(); n != nil {
		ref = n.Ref
	}
	t.lib = lib
	t.rebuild()
	if ref != "" {
		for i, r := range t.rows {
			if r.node.Ref == ref {
				t.cursor = i
				break
			}
		}
	}
	t.scroll()
}

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
	t.shown = nil
	if f := fold(s); f != "" {
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
	t.rebuild()
	t.cursor = 0
	if prev != nil {
		if i := t.indexOf(prev); i >= 0 {
			t.cursor = i
		}
	}
	t.scroll()
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
	cur := lipgloss.NewStyle().Foreground(t.pal.Gold).Reverse(true)

	lines := make([]string, t.h)
	for i := range lines {
		idx := t.offset + i
		if idx >= len(t.rows) {
			continue
		}
		r := t.rows[idx]
		marker := "  "
		if !r.node.Leaf {
			marker = "▸ "
			if t.shown != nil || t.expanded[r.node.Ref] {
				marker = "▾ "
			}
		}
		star := ""
		if r.node.Leaf && t.starred(r.node.Ref) {
			star = "★ "
		}
		indent := strings.Repeat("  ", r.depth)
		label := r.node.Label()

		if idx == t.cursor {
			s := truncate(indent+marker+star+label, t.w)
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
		put(label, normal)
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
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
