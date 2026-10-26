package render

import (
	"image/color"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"vedabase-tui/internal/api"
)

// Link is one clickable span of a rendered line. Columns are terminal cells of
// the line with ANSI sequences excluded; End is exclusive. A reference that
// Glamour wraps across lines yields one Link per line.
type Link struct {
	Line, Start, End int
	Ref              string
}

// Sentinels bracket a reference in the Markdown. They are format characters:
// zero width, passed through by goldmark and Glamour, never part of a word,
// and absent from the corpus (markRefs strips any that appear).
const (
	linkStart = "⁣" // INVISIBLE SEPARATOR
	linkEnd   = "⁤" // INVISIBLE PLUS
)

var sentinelStripper = strings.NewReplacer(linkStart, "", linkEnd, "")

type span struct {
	start, end int
	display    string
	ref        string
}

// markRefs wraps every reference of the given field found in text in
// sentinels and appends their targets to refs in order of appearance. Wiki
// links ([[ref|label]]) are matched by their full text and shown as their
// label; plain text must pass the boundary rules.
func markRefs(text, field string, references []api.Reference, refs *[]string) string {
	text = sentinelStripper.Replace(text)
	var spans []span
	for _, r := range references {
		if r.Ref == "" || r.Field != field || r.Text == "" {
			continue
		}
		wiki := strings.HasPrefix(r.Text, "[[")
		display := r.Text
		if wiki {
			display = inlineLinks(r.Text)
		}
		for from := 0; ; {
			i := strings.Index(text[from:], r.Text)
			if i < 0 {
				break
			}
			s, e := from+i, from+i+len(r.Text)
			from = s + 1
			for from < len(text) && !utf8.RuneStart(text[from]) {
				from++
			}
			if !wiki && !plainBoundary(text, s, e) {
				continue
			}
			spans = append(spans, span{s, e, display, r.Ref})
		}
	}
	if len(spans) == 0 {
		return text
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		if sp.start < last { // overlaps the previous span
			continue
		}
		b.WriteString(text[last:sp.start])
		b.WriteString(linkStart + sp.display + linkEnd)
		*refs = append(*refs, sp.ref)
		last = sp.end
	}
	b.WriteString(text[last:])
	return b.String()
}

// plainBoundary reports whether text[s:e] stands alone: not glued to a letter
// or digit before, and not continued by a digit (4.7 vs 4.70, 4.7-9, 4.7.1)
// after.
func plainBoundary(text string, s, e int) bool {
	if r, _ := utf8.DecodeLastRuneInString(text[:s]); s > 0 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
		return false
	}
	rest := text[e:]
	r, n := utf8.DecodeRuneInString(rest)
	if rest == "" {
		return true
	}
	if unicode.IsDigit(r) {
		return false
	}
	if r == '.' || r == '-' || r == '–' {
		if next, _ := utf8.DecodeRuneInString(rest[n:]); unicode.IsDigit(next) {
			return false
		}
	}
	return true
}

// applyLinks strips the sentinels from Glamour's output, underlines each span
// in the accent colour and records where it is. Lines keep their widths. The
// state in force before a span (colour, bold…) resumes after it.
func applyLinks(out string, refs []string, accent color.Color) (string, []Link) {
	if !strings.Contains(out, linkStart) {
		return sentinelStripper.Replace(out), nil
	}
	linkSGR := ansi.NewStyle().ForegroundColor(accent).Underline(true).String()
	lines := strings.Split(out, "\n")
	var links []Link
	var active []string // SGR sequences since the last reset
	open, next := false, 0
	ref := ""
	for ln, line := range lines {
		var b strings.Builder
		col := 0
		started := false // the underline is on for the current piece
		start, end := 0, 0
		var pend strings.Builder    // spaces (and styling) after the last visible char of the piece
		var pendSp strings.Builder  // just the spaces of pend
		var pendSty strings.Builder // just the styling of pend
		pendSGR := false
		closePiece := func() {
			if !started {
				return
			}
			b.WriteString("\x1b[0m" + strings.Join(active, ""))
			b.WriteString(pend.String())
			pend.Reset()
			pendSp.Reset()
			pendSty.Reset()
			pendSGR = false
			started = false
			if ref != "" {
				links = append(links, Link{ln, start, end, ref})
			}
		}
		state := byte(ansi.NormalState)
		for i := 0; i < len(line); {
			if m := strings.IndexAny(line[i:], linkStart+linkEnd); m != 0 {
				stop := len(line)
				if m > 0 {
					stop = i + m
				}
				chunk := line[i:stop]
				for len(chunk) > 0 {
					seq, w, n, ns := ansi.DecodeSequence(chunk, state, nil)
					state = ns
					chunk = chunk[n:]
					if w == 0 {
						if strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m") {
							if seq == "\x1b[m" || seq == "\x1b[0m" {
								active = active[:0]
							} else {
								active = append(active, seq)
							}
							if started {
								pend.WriteString(seq)
								pendSty.WriteString(seq)
								pendSGR = true
								continue
							}
						}
						if started {
							pend.WriteString(seq)
							pendSty.WriteString(seq)
						} else {
							b.WriteString(seq)
						}
						continue
					}
					if open && strings.TrimSpace(seq) == "" {
						if started {
							pend.WriteString(seq)
							pendSp.WriteString(seq)
						} else {
							b.WriteString(seq)
						}
						col += w
						continue
					}
					if open && !started {
						b.WriteString(linkSGR)
						started, start = true, col
					}
					if started {
						// interior spaces belong to the link: underline them, then
						// let any styling in between take effect for the next word
						if pendSp.Len() > 0 {
							b.WriteString(linkSGR + pendSp.String())
						}
						b.WriteString(pendSty.String())
						if pendSGR {
							b.WriteString(linkSGR) // an SGR in between may have changed the look
						}
						pend.Reset()
						pendSp.Reset()
						pendSty.Reset()
						pendSGR = false
						end = col + w
					}
					b.WriteString(seq)
					col += w
				}
				i = stop
				continue
			}
			if strings.HasPrefix(line[i:], linkStart) {
				i += len(linkStart)
				open, ref = true, ""
				if next < len(refs) {
					ref = refs[next]
				}
				next++
			} else {
				i += len(linkEnd)
				closePiece()
				open = false
			}
		}
		closePiece()
		lines[ln] = b.String()
	}
	return strings.Join(lines, "\n"), links
}
