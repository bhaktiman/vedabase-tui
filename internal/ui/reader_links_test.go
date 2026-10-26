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
