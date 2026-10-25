package render

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
)

func loadItem(t *testing.T, name string) api.Item {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var it api.Item
	if err := json.Unmarshal(b, &it); err != nil {
		t.Fatal(err)
	}
	return it
}

func TestDisplayMarkdownParts(t *testing.T) {
	it := loadItem(t, "verse-bg-2-13.json")
	md := DisplayMarkdown(it, api.AllParts())
	if !strings.HasPrefix(md, "## Bg 2.13") || !strings.Contains(md, "#### Word for word") || !strings.Contains(md, "dehinaḥ") {
		t.Fatalf("missing content:\n%s", md)
	}
	p := api.AllParts()
	p.Synonyms = false
	if md := DisplayMarkdown(it, p); strings.Contains(md, "dehinaḥ") || strings.Contains(md, "Word for word") {
		t.Fatalf("synonyms not hidden:\n%s", md)
	}
	l := DisplayMarkdown(loadItem(t, "verse-lcfl-1.json"), api.AllParts())
	if strings.Contains(l, "#### Devanagari") {
		t.Fatalf("empty part shown:\n%s", l)
	}
}

func TestDisplayMarkdownDocument(t *testing.T) {
	md := DisplayMarkdown(loadItem(t, "verse-letter-aatie.json"), api.AllParts())
	if !strings.HasPrefix(md, "## Letter to: Aatie") || !strings.Contains(md, "October 28th 1967 — Navadvipa") {
		t.Fatalf("bad document markdown:\n%.300s", md)
	}
}

func TestRenderCaches(t *testing.T) {
	r := NewRenderer(Default)
	md := DisplayMarkdown(loadItem(t, "verse-bg-2-13.json"), api.AllParts())
	a, err := r.Render("bg/2/13", md, 80, api.AllParts())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := r.Render("bg/2/13", md, 80, api.AllParts())
	if a != b || r.renders != 1 {
		t.Fatalf("second call not cached: renders=%d", r.renders)
	}
	r.Render("bg/2/13", md, 60, api.AllParts())
	if r.renders != 2 {
		t.Fatalf("different width should re-render, renders=%d", r.renders)
	}
	for i := 0; i < 40; i++ {
		r.Render("x", "hi", 10+i, api.AllParts())
	}
	if r.lru.Len() != cacheSize || len(r.entries) != cacheSize {
		t.Fatalf("lru size %d/%d", r.lru.Len(), len(r.entries))
	}
}

func TestRenderNoANSIWidthOverflow(t *testing.T) {
	r := NewRenderer(Default)
	md := DisplayMarkdown(loadItem(t, "verse-bg-2-13.json"), api.AllParts())
	out, err := r.Render("bg/2/13", md, 60, api.AllParts())
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line %d width %d > 60: %q", i, w, line)
		}
	}
}

func TestMarks(t *testing.T) {
	in := "a <mark>soul</mark> &amp; b<br>"
	if got := Marks(in, lipgloss.NewStyle()); got != "a soul & b" {
		t.Fatalf("plain: %q", got)
	}
	bold := lipgloss.NewStyle().Bold(true)
	want := bold.Render("soul")
	if want == "soul" {
		t.Skip("no colour profile forcing bold")
	}
	if got := Marks(in, bold); !strings.Contains(got, want) {
		t.Fatalf("bold: %q", got)
	}
}

func TestDisplayMarkdownProse(t *testing.T) {
	md := DisplayMarkdown(loadItem(t, "verse-lcfl-1.json"), api.AllParts())
	if !strings.HasPrefix(md, "## The First Morning Walk: April 18, 1973") || strings.Contains(md, "#### ") {
		t.Fatalf("bad prose markdown:\n%.300s", md)
	}
	if md2 := DisplayMarkdown(loadItem(t, "verse-lcfl-1.json"), api.Parts{}); md2 != md {
		t.Fatal("toggles should not affect prose")
	}
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestVerseLinesStaySeparate(t *testing.T) {
	r := NewRenderer(Default)
	md := DisplayMarkdown(loadItem(t, "verse-bg-2-13.json"), api.AllParts())
	out, err := r.Render("bg/2/13", md, 100, api.AllParts())
	if err != nil {
		t.Fatal(err)
	}
	line := func(sub string) int {
		for i, l := range strings.Split(ansiRe.ReplaceAllString(out, ""), "\n") {
			if strings.Contains(l, sub) {
				return i
			}
		}
		t.Fatalf("%q not found in:\n%s", sub, out)
		return -1
	}
	if a, b := line("dehino 'smin yathā dehe"), line("kaumāraṁ yauvanaṁ jarā"); a == b {
		t.Errorf("verse lines share row %d", a)
	}
	if a, b := line("देहिनोऽस्मिन्यथा देहे"), line("तथा देहान्तरप्राप्तिर्धीरस्तत्र"); a == b {
		t.Errorf("devanagari lines share row %d", a)
	}
}

func TestMarksStripsDecodedControls(t *testing.T) {
	got := Marks("a&#27;[2Jb <mark>x&#x9b;y</mark>", lipgloss.NewStyle())
	if strings.ContainsAny(got, "\x1b\u009b") {
		t.Fatalf("control character survived: %q", got)
	}
}
