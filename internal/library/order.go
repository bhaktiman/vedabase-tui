package library

import (
	"strconv"
	"strings"
	"unicode"
)

// Label is what the tree shows for n: its Title if known, else a default.
func (n *Node) Label() string {
	if n.Title != "" {
		return n.Title
	}
	p := n.Parent
	switch {
	case p == nil:
		return n.Book // roots get their book title via Title; unknown books show the code
	case n.Book == lettersCode || n.Book == transcriptsCode:
		return groupLabel(n)
	}
	if p.Parent == nil && !n.Leaf {
		switch {
		case n.Book == "cc":
			if s, ok := lilaLabels[n.Seg]; ok {
				return s
			}
		case n.Book == "sb" && isNumber(n.Seg):
			return "Canto " + n.Seg
		}
	}
	if isFrontMatter(n.Seg) {
		return "Front matter (" + n.Seg + ")"
	}
	if !isNumber(n.Seg) && !startsWithDigit(n.Seg) {
		return titleCase(n.Seg)
	}
	if n.Leaf && p.Parent != nil {
		parts := strings.Split(n.Ref, "/")[1:]
		if _, ok := lilaLabels[parts[0]]; ok && n.Book == "cc" && len(parts) > 1 {
			parts = parts[1:] // the lila shows on the ancestor
		}
		return strings.Join(parts, ".")
	}
	if n.Leaf && p.Parent == nil && root(p).flat {
		return "Text " + n.Seg
	}
	return "Chapter " + n.Seg
}

var lilaLabels = map[string]string{"adi": "Ādi-līlā", "madhya": "Madhya-līlā", "antya": "Antya-līlā"}

func groupLabel(n *Node) string {
	switch {
	case n.Ref == lettersYearRef:
		return "By year"
	case n.Ref == lettersToRef:
		return "By recipient"
	case strings.HasPrefix(n.Ref, lettersToRef+"/"):
		return titleCase(strings.ReplaceAll(n.Seg, "-", ", "))
	case strings.HasPrefix(n.Ref, lettersYearRef+"/"), strings.HasPrefix(n.Ref, transcriptsCode+"#"):
		return n.Seg
	}
	return n.Seg // letter or transcript slug until a Title is set
}

func isNumber(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

func startsWithDigit(s string) bool { return s != "" && s[0] >= '0' && s[0] <= '9' }

// isFrontMatter matches "0a", "0b", ...
func isFrontMatter(s string) bool {
	return len(s) == 2 && s[0] == '0' && s[1] >= 'a' && s[1] <= 'z'
}

// titleCase capitalizes each word, treating "_" and "-" as separators.
func titleCase(s string) string {
	prev := ' '
	return strings.Map(func(r rune) rune {
		out := r
		if prev == ' ' || prev == ',' {
			out = unicode.ToUpper(r)
		}
		prev = r
		if r == '_' || r == '-' {
			prev, out = ' ', ' '
		}
		return out
	}, s)
}

func root(n *Node) *Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}

// Next returns the leaf after n in reading order, or nil at the end. Within a
// book it crosses chapters; in letters and transcripts it stays inside the
// node's group.
func (l *Library) Next(n *Node) *Node { return l.step(n, 1) }

// Prev is the mirror of Next.
func (l *Library) Prev(n *Node) *Node { return l.step(n, -1) }

func (l *Library) step(n *Node, d int) *Node {
	if n == nil || !n.Leaf {
		return nil
	}
	if n.Book == lettersCode || n.Book == transcriptsCode {
		sib := n.Parent.Children
		for i := range sib {
			if sib[i] == n {
				for j := i + d; j >= 0 && j < len(sib); j += d {
					if sib[j].Leaf {
						return sib[j]
					}
				}
				return nil
			}
		}
		return nil
	}
	i, ok := l.flatAt[n]
	if !ok {
		return nil
	}
	flat := l.flat[n.Book]
	if j := i + d; j >= 0 && j < len(flat) {
		return flat[j]
	}
	return nil
}

// Position is n's 1-based index among its leaf siblings, and their count.
func Position(n *Node) (idx, total int) {
	if n == nil || n.Parent == nil || !n.Leaf {
		return 0, 0
	}
	for _, s := range n.Parent.Children {
		if !s.Leaf {
			continue
		}
		total++
		if s == n {
			idx = total
		}
	}
	return idx, total
}
