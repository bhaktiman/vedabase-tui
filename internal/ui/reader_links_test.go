package ui

import (
	"testing"

	"vedabase-tui/internal/render"
)

func TestReaderLinksFollowTrimmedLines(t *testing.T) {
	var r Reader
	r.SetContent("\n\nfirst\nsecond", []render.Link{{Line: 3, Start: 0, End: 6, Ref: "x"}, {Line: 0, Ref: "gone"}}, false)
	if len(r.links) != 1 || r.links[0].Line != 1 || r.lines[1] != "second" {
		t.Fatalf("links = %+v lines = %q", r.links, r.lines)
	}
	r.Clear()
	if len(r.links) != 0 {
		t.Fatal("Clear kept links")
	}
}

func linkReader() *Reader {
	var r Reader
	r.SetContent("l0\nl1\nl2\nl3\nl4\nl5", []render.Link{
		{Line: 1, Start: 0, End: 4, Ref: "a"}, {Line: 3, Start: 2, End: 9, Ref: "b"},
		{Line: 3, Start: 12, End: 15, Ref: "c"}, {Line: 5, Start: 0, End: 2, Ref: "d"},
	}, false)
	r.SetSize(8, 3)
	return &r
}

func TestLinkAt(t *testing.T) {
	r := linkReader()
	r.offset = 1 // rows show lines 1..3
	for _, c := range []struct {
		row, col int
		ref      string
	}{{0, 0, "a"}, {0, 3, "a"}, {0, 4, ""}, {2, 2, "b"}, {2, 7, "b"}, {2, 8, ""}, {2, 12, ""}, {1, 0, ""}, {3, 0, ""}, {-1, 0, ""}, {0, -1, ""}} {
		l, ok := r.LinkAt(c.row, c.col)
		if ok != (c.ref != "") || l.Ref != c.ref {
			t.Errorf("LinkAt(%d,%d) = %+v, %v; want %q", c.row, c.col, l, ok, c.ref)
		}
	}
}

func TestVisibleLinksClipToWidth(t *testing.T) {
	r := linkReader()
	r.offset = 1
	got := r.VisibleLinks()
	if len(got) != 2 || got[0].Ref != "a" || got[1].Ref != "b" || got[1].End != 8 {
		t.Fatalf("VisibleLinks = %+v", got)
	}
	r.offset = 3
	if got := r.VisibleLinks(); len(got) != 2 || got[0].Ref != "b" || got[1].Ref != "d" {
		t.Fatalf("scrolled VisibleLinks = %+v", got)
	}
}
