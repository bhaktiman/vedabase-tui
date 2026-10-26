package render

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"vedabase-tui/internal/api"
)

func linkedItem(purport string, refs ...api.Reference) api.Item {
	return api.Item{Ref: "sb/1/1/1", Translation: "Tr", Purport: purport, References: refs}
}

func TestMarkRefsBoundaries(t *testing.T) {
	refs := []api.Reference{{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "purport"}}
	text := "Bg. 4.70 no; xBg. 4.7 no; Bg. 4.7-9 no; Bg. 4.7.1 no; Bg. 4.7–9 no; (Bg. 4.7) yes; Bg. 4.7. yes; Bg. 4.7"
	var got []string
	out := markRefs(text, "purport", refs, &got)
	if n := strings.Count(out, linkStart); n != 3 || len(got) != 3 {
		t.Fatalf("want 3 links, got %d (%v):\n%s", n, got, out)
	}
	if !strings.Contains(out, "("+linkStart+"Bg. 4.7"+linkEnd+")") {
		t.Fatalf("out = %q", out)
	}
	// a field that is not the reference's own is left alone
	got = nil
	if out := markRefs(text, "translation", refs, &got); out != text || got != nil {
		t.Fatal("marked the wrong field")
	}
}

func TestDisplayLinkedMatching(t *testing.T) {
	it := linkedItem("See [[bg/7/1|*Bg.* 7.1]], Bg. 4.7, SN 1.1 and Bg. 4.70.",
		api.Reference{Text: "[[bg/7/1|*Bg.* 7.1]]", Ref: "bg/7/1", Field: "purport"},
		api.Reference{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "purport"},
		api.Reference{Text: "SN 1.1", Ref: "", Field: "purport"})
	md, refs := DisplayLinked(it, api.AllParts())
	if strings.Join(refs, ",") != "bg/7/1,bg/4/7" {
		t.Fatalf("refs = %v", refs)
	}
	want := "See " + linkStart + "*Bg.* 7.1" + linkEnd + ", " + linkStart + "Bg. 4.7" + linkEnd + ", SN 1.1 and Bg. 4.70."
	if !strings.Contains(md, want) {
		t.Fatalf("md = %q", md)
	}
	if strings.Contains(DisplayMarkdown(it, api.AllParts()), linkStart) {
		t.Fatal("DisplayMarkdown must stay free of sentinels")
	}
	// hidden parts have no links
	it.VerseText = "a verse"
	p := api.AllParts()
	p.Purport = false
	if md, refs := DisplayLinked(it, p); len(refs) != 0 || strings.Contains(md, linkStart) {
		t.Fatalf("link in a hidden part: %v %q", refs, md)
	}
}

func cells(line string, l Link) string { return ansi.Cut(ansi.Strip(line), l.Start, l.End) }

func TestRenderRecordsLinks(t *testing.T) {
	it := linkedItem("See [[bg/7/1|Bg. 7.1]] and Bg. 4.7.",
		api.Reference{Text: "[[bg/7/1|Bg. 7.1]]", Ref: "bg/7/1", Field: "purport"},
		api.Reference{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "purport"})
	md, refs := DisplayLinked(it, api.AllParts())
	out, links, err := NewRenderer(Default).Render("x", "en", md, refs, 80, api.AllParts())
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 || links[0].Ref != "bg/7/1" || links[1].Ref != "bg/4/7" {
		t.Fatalf("links = %+v", links)
	}
	lines := strings.Split(out, "\n")
	for i, want := range []string{"Bg. 7.1", "Bg. 4.7"} {
		if got := cells(lines[links[i].Line], links[i]); got != want {
			t.Errorf("link %d covers %q, want %q", i, got, want)
		}
	}
	if strings.ContainsAny(out, linkStart+linkEnd) || strings.Contains(ansi.Strip(out), "[[") {
		t.Fatal("sentinels or wiki brackets left in the output")
	}
	if !strings.Contains(lines[links[0].Line], "\x1b[4") && !strings.Contains(lines[links[0].Line], ";4") {
		t.Errorf("link not underlined: %q", lines[links[0].Line])
	}
}

func TestLinkWrappedAcrossLines(t *testing.T) {
	ref := api.Reference{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "purport"}
	for pad := 0; pad < 40; pad++ {
		it := linkedItem(strings.Repeat("x", pad)+" see Bg. 4.7 and more words after it", ref)
		md, refs := DisplayLinked(it, api.AllParts())
		out, links, err := NewRenderer(Default).Render("x", "en", md, refs, 40, api.AllParts())
		if err != nil {
			t.Fatal(err)
		}
		if len(links) != 2 {
			continue
		}
		lines := strings.Split(out, "\n")
		if links[1].Line != links[0].Line+1 || links[0].Ref != links[1].Ref {
			t.Fatalf("links = %+v", links)
		}
		a, b := cells(lines[links[0].Line], links[0]), cells(lines[links[1].Line], links[1])
		if a != "Bg." || b != "4.7" {
			t.Fatalf("pieces %q %q, links %+v\n%s", a, b, links, ansi.Strip(out))
		}
		return
	}
	t.Fatal("never produced a wrapped link")
}

// stripped of styling, linked output equals the no-link render, line by line.
func TestLinksDoNotChangeText(t *testing.T) {
	for _, name := range []string{"verse-sb-1-1-1.json", "verse-lecture-680317bgsf.json", "660219bg-new-york.json"} {
		for _, w := range []int{40, 80} {
			it := loadItem(t, name)
			if name == "660219bg-new-york.json" {
				it.References = []api.Reference{{Text: "Kṛṣṇa", Ref: "x/1", Field: "purport"}, {Text: "Arjuna", Ref: "x/2", Field: "purport"}}
			}
			bare := it
			bare.References = nil
			md, refs := DisplayLinked(it, api.AllParts())
			r := NewRenderer(Default)
			got, links, err := r.Render("a", "en", md, refs, w, api.AllParts())
			if err != nil {
				t.Fatal(err)
			}
			want, _, _ := r.Render("b", "en", DisplayMarkdown(bare, api.AllParts()), nil, w, api.AllParts())
			if len(links) == 0 {
				t.Fatalf("%s: no links", name)
			}
			gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
			if len(gl) != len(wl) {
				t.Fatalf("%s w=%d: %d lines vs %d", name, w, len(gl), len(wl))
			}
			for i := range gl {
				if ansi.Strip(gl[i]) != ansi.Strip(wl[i]) {
					t.Fatalf("%s w=%d line %d:\n%q\n%q", name, w, i, ansi.Strip(gl[i]), ansi.Strip(wl[i]))
				}
			}
			for _, l := range links {
				if l.Start >= l.End || l.End > ansi.StringWidth(gl[l.Line]) {
					t.Fatalf("%s: bad link %+v", name, l)
				}
			}
		}
	}
}

func TestApplyLinksRestoresStyle(t *testing.T) {
	in := "\x1b[38;5;1mfoo " + linkStart + "bar" + linkEnd + " baz\x1b[0m"
	out, links := applyLinks(in, []string{"r"}, Default.Accent)
	if len(links) != 1 || links[0].Start != 4 || links[0].End != 7 {
		t.Fatalf("links = %+v", links)
	}
	i := strings.Index(out, "bar")
	if !strings.HasPrefix(out[i+3:], "\x1b[0m\x1b[38;5;1m baz") {
		t.Fatalf("style not restored after the link: %q", out)
	}
	if ansi.Strip(out) != "foo bar baz" {
		t.Fatalf("text = %q", ansi.Strip(out))
	}
}

func TestLinksOnLongLecture(t *testing.T) {
	it := loadItem(t, "660219bg-new-york.json")
	if len(it.Purport) < 100000 {
		t.Fatalf("fixture only %d chars", len(it.Purport))
	}
	it.References = []api.Reference{{Text: "Kṛṣṇa", Ref: "x/1", Field: "purport"}, {Text: "Arjuna", Ref: "x/2", Field: "purport"}, {Text: "Bhagavad-gītā", Ref: "bg", Field: "purport"}}
	bare := it
	bare.References = nil
	best := func(f func()) time.Duration {
		d := time.Duration(1 << 62)
		for i := 0; i < 3; i++ {
			s := time.Now()
			f()
			d = min(d, time.Since(s))
		}
		return d
	}
	var n int
	plain := best(func() {
		NewRenderer(Default).Render("a", "en", DisplayMarkdown(bare, api.AllParts()), nil, 80, api.AllParts())
	})
	linked := best(func() {
		md, refs := DisplayLinked(it, api.AllParts())
		_, links, _ := NewRenderer(Default).Render("a", "en", md, refs, 80, api.AllParts())
		n = len(links)
	})
	t.Log(fmt.Sprintf("%d chars, %d link pieces: plain %v, linked %v", len(it.Purport), n, plain, linked))
	if n < 50 {
		t.Fatalf("only %d links", n)
	}
	if linked > 2*plain {
		t.Fatalf("linked %v > 2x plain %v", linked, plain)
	}
}

func TestInteriorSpaceStaysInOnePiece(t *testing.T) {
	g := "\x1b[38;5;252m"
	accent := Default.Accent
	for name, in := range map[string]string{
		"per-word SGR":   g + "(" + linkStart + "Bg.\x1b[0m" + g + " " + g + "4.7" + linkEnd + g + ")\x1b[0m",
		"reset in space": g + "(" + linkStart + "Bg.\x1b[0m " + g + "4.7" + linkEnd + ")\x1b[0m",
		"sgr only":       "(" + linkStart + g + "Bg." + g + " " + g + "4.7\x1b[0m" + linkEnd + ")",
	} {
		out, links := applyLinks(in, []string{"bg/4/7"}, accent)
		if len(links) != 1 {
			t.Fatalf("%s: want one link, got %+v\n%q", name, links, out)
		}
		if got := cells(out, links[0]); got != "Bg. 4.7" {
			t.Fatalf("%s: link covers %q", name, got)
		}
		ul := underlined(out)
		for c := links[0].Start; c < links[0].End; c++ {
			if !ul[c] {
				t.Fatalf("%s: cell %d not underlined: %q", name, c, out)
			}
		}
		if ul[links[0].End] {
			t.Fatalf("%s: underline runs past the link: %q", name, out)
		}
	}
}

// underlined reports, per visible cell of s, whether the SGR state has
// underline on (a parameter 4 outside colour arguments is enough here).
func underlined(s string) []bool {
	var res []bool
	on := false
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "\x1b[") {
			e := strings.IndexByte(s[i:], 'm')
			params := s[i+2 : i+e]
			if params == "" || params == "0" {
				on = false
			}
			for _, p := range strings.Split(params, ";") {
				if p == "4" {
					on = true
				}
			}
			i += e + 1
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		res = append(res, on)
		i += n
	}
	return append(res, false)
}

func TestStraySentinelsInOtherFieldsDoNotShiftLinks(t *testing.T) {
	it := linkedItem("See Bg. 4.7 now.", api.Reference{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "purport"})
	it.Synonyms = "mark" + linkStart + " word"
	it.VerseText = "a" + linkEnd + "b"
	it.Devanagari = linkStart + "x"
	it.Document = &api.Document{Date: "1968" + linkStart, Location: linkEnd + "LA"}
	md, refs := DisplayLinked(it, api.AllParts())
	if len(refs) != 1 || strings.Count(md, linkStart) != 1 || strings.Count(md, linkEnd) != 1 {
		t.Fatalf("refs=%v md=%q", refs, md)
	}
	if plain := DisplayMarkdown(it, api.AllParts()); strings.ContainsAny(plain, linkStart+linkEnd) {
		t.Fatalf("DisplayMarkdown kept a sentinel: %q", plain)
	}
	if it.Document.Date != "1968"+linkStart {
		t.Fatal("the caller's document was modified")
	}
}
