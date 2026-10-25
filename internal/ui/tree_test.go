package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/library"
	"vedabase-tui/internal/render"
)

func newTestTree(t *testing.T) (Tree, *library.Library) {
	t.Helper()
	lib := sampleLibrary(t)
	tr := NewTree(lib, func(string) bool { return false }, render.Default)
	tr.SetSize(40, 20)
	return tr, lib
}

// press feeds keys to the tree and returns the last command.
func press(tr Tree, keys ...string) (Tree, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		tr, cmd = tr.Update(key(k))
	}
	return tr, cmd
}

func labels(tr Tree) []string {
	var out []string
	for _, r := range tr.rows {
		out = append(out, r.node.Label())
	}
	return out
}

func has(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestTreeExpandAndOpen(t *testing.T) {
	tr, _ := newTestTree(t)
	if got := tr.Selected().Label(); got != "★ Collections" {
		t.Fatalf("start on %q", got)
	}
	tr, _ = press(tr, "j")
	if got := tr.Selected().Label(); got != "Press s on a verse to start one" {
		t.Fatalf("empty collections show %q", got)
	}
	tr, _ = press(tr, "j")
	if got := tr.Selected().Label(); got != "Bhagavad-gītā As It Is" {
		t.Fatalf("then %q", got)
	}
	tr, _ = press(tr, "l")
	if !has(labels(tr), "Chapter 1") {
		t.Fatalf("rows %v lack Chapter 1", labels(tr))
	}
	tr, _ = press(tr, "j", "j")
	if got := tr.Selected().Label(); got != "Chapter 1" {
		t.Fatalf("on %q", got)
	}
	tr, _ = press(tr, "l", "j")
	if got := tr.Selected().Label(); got != "1.1" {
		t.Fatalf("on %q", got)
	}
	_, cmd := press(tr, "l")
	if cmd == nil {
		t.Fatal("no cmd")
	}
	m, ok := cmd().(OpenMsg)
	if !ok || m.Node.Ref != "bg/1/1" {
		t.Fatalf("got %#v", cmd())
	}
}

func TestTreeCollapseToParent(t *testing.T) {
	tr, lib := newTestTree(t)
	tr.Reveal(lib.Find("bg/1/1"))
	tr, _ = press(tr, "h")
	if got := tr.Selected().Ref; got != "bg/1" {
		t.Fatalf("on %q", got)
	}
	if !has(labels(tr), "1.1") {
		t.Fatal("bg/1 should still be expanded")
	}
	tr, _ = press(tr, "h")
	if has(labels(tr), "1.1") {
		t.Fatal("bg/1 should be collapsed")
	}
}

func TestTreeGGAndG(t *testing.T) {
	tr, _ := newTestTree(t)
	tr, _ = press(tr, "G")
	if got := tr.Selected().Label(); got != "Lectures and conversations" {
		t.Fatalf("G on %q", got)
	}
	tr, _ = press(tr, "g", "g")
	if got := tr.Selected().Ref; got != "collections#" {
		t.Fatalf("gg on %q", got)
	}
}

func TestTreeReveal(t *testing.T) {
	tr, lib := newTestTree(t)
	tr.SetSize(40, 3)
	n := lib.Find("cc/madhya/9/74")
	tr.Reveal(n)
	if tr.Selected() == nil || tr.Selected().Ref != "cc/madhya/9/74" {
		t.Fatalf("selected %v", tr.Selected())
	}
	for p := n.Parent; p != nil; p = p.Parent {
		if !tr.expanded[p.Ref] {
			t.Errorf("%s not expanded", p.Ref)
		}
	}
	if !strings.Contains(tr.View(), "9.74") {
		t.Errorf("cursor row not scrolled into view:\n%s", tr.View())
	}
}

func TestTreeStarred(t *testing.T) {
	lib := sampleLibrary(t)
	tr := NewTree(lib, func(r string) bool { return r == "bg/2/13" }, render.Default)
	tr.SetSize(40, 20)
	tr.Reveal(lib.Find("bg/2/13"))
	tr, _ = press(tr, "j") // move the cursor off, so the row is plain
	var line string
	for _, l := range strings.Split(tr.View(), "\n") {
		if strings.Contains(l, "2.13") {
			line = l
		}
	}
	if !strings.Contains(line, "★") {
		t.Fatalf("no star in %q", line)
	}
	if v := strings.ReplaceAll(tr.View(), "★ Collections", ""); strings.Count(v, "★") != 1 {
		t.Fatal("more than one star")
	}
}

func TestTreeFilter(t *testing.T) {
	tr, _ := newTestTree(t)
	full := len(tr.rows)
	tr.SetFilter("abhi")
	var leaves []string
	for _, r := range tr.rows {
		if r.node.Leaf {
			leaves = append(leaves, r.node.Ref)
			if r.node.Parent.Label() != "Abhirama" {
				t.Errorf("leaf %s not under Abhirama", r.node.Ref)
			}
		}
	}
	if len(leaves) == 0 {
		t.Fatalf("no leaves: %v", labels(tr))
	}
	if !has(labels(tr), "Abhirama") || !has(labels(tr), "By recipient") {
		t.Errorf("matches/ancestors missing: %v", labels(tr))
	}
	tr.SetFilter("ABHÍ") // case and diacritics fold
	if len(tr.rows) == 0 {
		t.Error("folded filter matched nothing")
	}
	tr.SetFilter("")
	if len(tr.rows) != full {
		t.Errorf("rows %d, want %d", len(tr.rows), full)
	}
}

func TestTreeLazy(t *testing.T) {
	tr, lib := newTestTree(t)
	tr.Reveal(lib.Find("letters#year"))
	_, cmd := press(tr, "l")
	if cmd == nil {
		t.Fatal("no cmd")
	}
	m, ok := cmd().(ExpandLazyMsg)
	if !ok || m.Node.Ref != "letters#year" {
		t.Fatalf("got %#v", cmd())
	}
}

func TestTreeNarrow(t *testing.T) {
	tr, lib := newTestTree(t)
	tr.Reveal(lib.Find("bg/1/16-18"))
	tr.SetSize(10, 3)
	for _, line := range strings.Split(tr.View(), "\n") {
		if w := lipgloss.Width(line); w > 10 {
			t.Errorf("line %q width %d", line, w)
		}
	}
	tr.SetSize(1, 1)
	tr.View()
	tr.SetSize(0, 0)
	tr.View()
}

func TestKeySeq(t *testing.T) {
	var k keySeq
	for _, c := range []struct{ in, want string }{
		{"g", ""}, {"j", "j"}, {"g", ""}, {"g", "top"}, {"G", "bottom"}, {"ctrl+d", "halfdown"}, {"ctrl+u", "halfup"}, {"enter", "enter"},
	} {
		if got := k.Resolve(key(c.in)); got != c.want {
			t.Errorf("%s: got %q want %q", c.in, got, c.want)
		}
	}
}
