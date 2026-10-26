package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/render"
)

// Reader scrolls an already-rendered document. Rendering happens elsewhere
// (in a background Cmd); this only slices lines.
type Reader struct {
	lines  []string
	links  []render.Link // Line indexes lines
	offset int
	w, h   int
}

func (r *Reader) SetSize(w, h int) {
	r.w, r.h = w, h
	r.clamp()
}

func (r *Reader) clamp() {
	r.offset = min(max(r.offset, 0), max(len(r.lines)-r.h, 0))
}

// Clear drops the content and scrolls to the top.
func (r *Reader) Clear() { r.lines, r.links, r.offset = nil, nil, 0 }

func (r *Reader) Empty() bool { return len(r.lines) == 0 }

// SetContent replaces the text. keepPlace keeps the same relative position,
// for a re-render of the same document at another width. links locate the
// references in out's lines (as returned by render).
func (r *Reader) SetContent(out string, links []render.Link, keepPlace bool) {
	old := len(r.lines)
	lead := len(out) - len(strings.TrimLeft(out, "\n"))
	r.lines = strings.Split(strings.Trim(out, "\n"), "\n")
	r.links = r.links[:0:0]
	for _, l := range links {
		if l.Line -= lead; l.Line >= 0 && l.Line < len(r.lines) {
			r.links = append(r.links, l)
		}
	}
	if keepPlace && old > 0 {
		r.offset = r.offset * len(r.lines) / old
	} else {
		r.offset = 0
	}
	r.clamp()
}

// Key scrolls for a logical key (see keySeq.Resolve) and reports whether it
// was a scroll key.
func (r *Reader) Key(k string) bool {
	half := max(r.h/2, 1)
	switch k {
	case "j", "down":
		r.offset++
	case "k", "up":
		r.offset--
	case "halfdown":
		r.offset += half
	case "halfup":
		r.offset -= half
	case "top":
		r.offset = 0
	case "bottom":
		r.offset = len(r.lines)
	default:
		return false
	}
	r.clamp()
	return true
}

// Lines returns exactly h lines, each cut to w cells.
func (r *Reader) Lines() []string {
	out := make([]string, r.h)
	cut := lipgloss.NewStyle().MaxWidth(max(r.w, 1))
	for i := range out {
		if j := r.offset + i; j < len(r.lines) {
			out[i] = cut.Render(r.lines[j])
		}
	}
	return out
}

// Total is the number of lines in the document; Offset the first one shown.
func (r *Reader) Total() int  { return len(r.lines) }
func (r *Reader) Offset() int { return r.offset }

// LinkAt is the link under the cell at col of visible row (0 is the first row
// shown), if any. Cells past the reader's width are cut off, so are not links.
func (r *Reader) LinkAt(row, col int) (render.Link, bool) {
	if row < 0 || row >= r.h || col < 0 || col >= r.w {
		return render.Link{}, false
	}
	line := r.offset + row
	for _, l := range r.links {
		if l.Line == line && col >= l.Start && col < l.End {
			return l, true
		}
	}
	return render.Link{}, false
}

// VisibleLinks are the links on the rows shown, in reading order, their
// columns clipped to the width. Line is still the index into the document;
// the row on screen is Line - Offset().
func (r *Reader) VisibleLinks() []render.Link {
	var out []render.Link
	for _, l := range r.links {
		if l.Line < r.offset || l.Line >= r.offset+r.h || l.Start >= r.w {
			continue
		}
		l.End = min(l.End, r.w)
		if l.End > l.Start {
			out = append(out, l)
		}
	}
	return out
}
