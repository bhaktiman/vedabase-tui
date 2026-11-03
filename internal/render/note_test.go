package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"vedabase-tui/internal/api"
)

func TestWithNoteShowsTextLiterally(t *testing.T) {
	it := linkedItem("See Bg. 4.7 here.", api.Reference{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "purport"})
	md, refs := DisplayLinked(it, api.AllParts())
	note := "**bold** and [[bg/1/1|Bg 1.1]] and [x](http://e.com)\n\nsecond " + linkStart + "para" + linkEnd + "\n1. not a list\n# not a heading"
	out, links, err := NewRenderer(Default).Render("sb/1/1/1", "en", WithNote(md, note), refs, 80, api.AllParts())
	if err != nil {
		t.Fatal(err)
	}
	txt := ansi.Strip(out)
	for _, want := range []string{"Your note", "**bold**", "[[bg/1/1|Bg 1.1]]", "[x](http://e.com)", "second para", "1. not a list", "# not a heading"} {
		if !strings.Contains(txt, want) {
			t.Errorf("output lacks %q:\n%s", want, txt)
		}
	}
	if len(links) != 1 || links[0].Ref != "bg/4/7" {
		t.Fatalf("links = %+v", links)
	}
	lines := strings.Split(out, "\n")
	if got := cells(lines[links[0].Line], links[0]); got != "Bg. 4.7" {
		t.Errorf("link covers %q", got)
	}
}

func TestWithNoteEmptyIsUnchanged(t *testing.T) {
	if WithNote("md", " \n ") != "md" {
		t.Fatal("blank note changed the page")
	}
}

func TestRenderCacheSeesTheNote(t *testing.T) {
	r := NewRenderer(Default)
	a, _, _ := r.Render("x", "en", "## T\n\nbody", nil, 80, api.AllParts())
	b, _, _ := r.Render("x", "en", WithNote("## T\n\nbody", "mine"), nil, 80, api.AllParts())
	if a == b || !strings.Contains(ansi.Strip(b), "mine") {
		t.Fatal("a note did not change the render")
	}
}
