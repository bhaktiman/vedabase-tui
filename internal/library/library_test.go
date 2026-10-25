package library

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"vedabase-tui/internal/api"
)

func testBooks() []api.Book {
	var bs []api.Book
	for _, c := range []string{"bg", "sb", "cc", "iso", "lcfl", "letters", "transcripts"} {
		b := api.Book{Code: c, Title: strings.ToUpper(c) + " title"}
		if c == "iso" {
			b.Type = "flat"
		}
		bs = append(bs, b)
	}
	return bs
}

func load(t *testing.T) *Library {
	t.Helper()
	f, err := os.Open("testdata/sitemap-sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	l, err := Parse(f, testBooks())
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func refs(ns []*Node) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Ref)
	}
	return out
}

func segs(ns []*Node) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Seg)
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRootsOrder(t *testing.T) {
	l := load(t)
	eq(t, refs(l.Roots), []string{"bg", "sb", "cc", "iso", "lcfl", "zzz", "letters", "transcripts"})
	if got := l.Find("zzz").Label(); got != "zzz" {
		t.Errorf("unknown book label = %q", got)
	}
	if got := l.Find("bg").Label(); got != "BG title" {
		t.Errorf("book label = %q", got)
	}
}

func TestOnlyBooksInSitemapGetRoots(t *testing.T) {
	l, err := Parse(strings.NewReader(`<urlset><url><loc>https://vedabase.cc/bg/1/</loc></url></urlset>`), testBooks())
	if err != nil {
		t.Fatal(err)
	}
	eq(t, refs(l.Roots), []string{"bg"})
}

func TestLeavesAndContainers(t *testing.T) {
	l := load(t)
	for ref, want := range map[string]bool{"bg/2": false, "bg/2/13": true, "lcfl/1": true, "bg/0a": true} {
		if got := l.Find(ref).Leaf; got != want {
			t.Errorf("%s Leaf = %v, want %v", ref, got, want)
		}
	}
	eq(t, segs(l.Find("bg").Children), []string{"0a", "1", "2", "10"})
	eq(t, segs(l.Find("cc").Children), []string{"adi", "madhya", "antya"})
	eq(t, segs(l.Find("bg/1").Children), []string{"1", "16-18", "19"})
}

func TestReadingOrderCrossesChapters(t *testing.T) {
	l := load(t)
	if got := l.Next(l.Find("bg/1/19")).Ref; got != "bg/2/1" {
		t.Errorf("next = %s", got)
	}
	if got := l.Prev(l.Find("bg/2/1")).Ref; got != "bg/1/19" {
		t.Errorf("prev = %s", got)
	}
	if got := l.Next(l.Find("bg/0a")).Ref; got != "bg/1/1" {
		t.Errorf("next = %s", got)
	}
	if got := l.Next(l.Find("bg/10/1")); got != nil {
		t.Errorf("end of book next = %v", got.Ref)
	}
	if got := l.Prev(l.Find("bg/0a")); got != nil {
		t.Errorf("start of book prev = %v", got.Ref)
	}
	if got := l.Next(l.Find("cc/adi/1/1")).Ref; got != "cc/madhya/9/74" {
		t.Errorf("next = %s", got)
	}
}

func TestPosition(t *testing.T) {
	l := load(t)
	if i, n := Position(l.Find("bg/1/16-18")); i != 2 || n != 3 {
		t.Errorf("Position = %d,%d", i, n)
	}
}

func TestLettersByRecipient(t *testing.T) {
	l := load(t)
	letters := l.Find("letters")
	eq(t, refs(letters.Children), []string{"letters#year", "letters#to"})
	year := l.Find("letters#year")
	if !year.Lazy || len(year.Children) != 0 {
		t.Errorf("year lazy=%v children=%d", year.Lazy, len(year.Children))
	}
	to := l.Find("letters#to")
	var labels []string
	for _, g := range to.Children {
		labels = append(labels, g.Label())
	}
	eq(t, labels, []string{"Aatie", "Abhirama"})
	eq(t, refs(l.Find("letters#to/abhirama").Children),
		[]string{"letters/letter-to-abhirama", "letters/letter-to-abhirama-2"})
	if got := l.Find("letters/letter-to-aatie").Parent.Ref; got != "letters#to/aatie" {
		t.Errorf("parent = %s", got)
	}
	if got := l.Next(l.Find("letters/letter-to-abhirama")).Ref; got != "letters/letter-to-abhirama-2" {
		t.Errorf("next = %s", got)
	}
	if got := l.Next(l.Find("letters/letter-to-abhirama-2")); got != nil {
		t.Errorf("last next = %v", got.Ref)
	}
	if got := l.Find("letters/letter-to-aatie").Label(); got != "letter-to-aatie" {
		t.Errorf("label = %q", got)
	}
}

func TestTranscriptsByYear(t *testing.T) {
	l := load(t)
	eq(t, segs(l.Find("transcripts").Children), []string{"1966", "1967"})
	if got := l.Next(l.Find("transcripts/660219bg-new-york")); got != nil {
		t.Errorf("next = %v", got.Ref)
	}
	if got := l.Find("transcripts/660219bg-new-york").Parent.Label(); got != "1966" {
		t.Errorf("year label = %q", got)
	}
}

func TestLabels(t *testing.T) {
	l := load(t)
	for ref, want := range map[string]string{
		"bg/2":           "Chapter 2",
		"sb/1":           "Canto 1",
		"cc/adi":         "Ādi-līlā",
		"cc/adi/1":       "Chapter 1",
		"bg/2/13":        "2.13",
		"lcfl/1":         "Chapter 1",
		"iso/1":          "Text 1",
		"cc/adi/1/1":     "1.1",
		"cc/madhya/9/74": "9.74",
		"sb/1/1/1":       "1.1.1",
		"bg/1/16-18":     "1.16-18",
		"zzz/1":          "Chapter 1",
		"bg/0a":          "Front matter (0a)",
		"sb/4/29/1b":     "4.29.1b",
	} {
		if got := l.Find(ref).Label(); got != want {
			t.Errorf("%s label = %q, want %q", ref, got, want)
		}
	}
	n := l.Find("bg/2")
	n.Title = "Contents of the Gītā Summarized"
	if n.Label() != n.Title {
		t.Errorf("title does not override: %q", n.Label())
	}
	if got := (&Node{Seg: "concluding_words", Leaf: true, Parent: &Node{Parent: &Node{}}, Book: "bg"}).Label(); got != "Concluding Words" {
		t.Errorf("concluding_words = %q", got)
	}
}

func TestAddYearGroup(t *testing.T) {
	l := load(t)
	g1 := l.AddYearGroup(1970, []string{"letters/letter-to-aatie"}, nil)
	g := l.AddYearGroup(1969, []string{"letters/letter-to-abhirama", "letters/letter-to-new"},
		map[string]string{"letters/letter-to-abhirama": "Letter to: Abhirāma"})
	if g1 == nil || g == nil || g.Ref != "letters#year/1969" || g.Parent.Ref != "letters#year" {
		t.Fatalf("group = %+v", g)
	}
	eq(t, segs(l.Find("letters#year").Children), []string{"1969", "1970"})
	eq(t, refs(g.Children), []string{"letters/letter-to-abhirama", "letters/letter-to-new"})
	if g.Children[0].Title != "Letter to: Abhirāma" || l.Find("letters/letter-to-abhirama").Title != "Letter to: Abhirāma" {
		t.Error("title not applied to both nodes")
	}
	if l.Find("letters/letter-to-abhirama").Parent.Ref != "letters#to/abhirama" {
		t.Error("Find no longer returns the By-recipient node")
	}
	if again := l.AddYearGroup(1969, []string{"letters/letter-to-abhirama"}, nil); again != g {
		t.Error("second call did not return the same group")
	}
	if len(g.Children) != 2 {
		t.Errorf("duplicate leaves: %d", len(g.Children))
	}
	if got := l.Next(g.Children[0]); got != g.Children[1] {
		t.Error("next within year group")
	}
}

func TestParseRealScale(t *testing.T) {
	f, err := os.Open("testdata/sitemap-en-books.xml")
	if err != nil {
		t.Skip("testdata/sitemap-en-books.xml not present")
	}
	defer f.Close()
	start := time.Now()
	l, err := Parse(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("parse took %v", d)
	}
	count := 0
	var walk func(n *Node)
	walk = func(n *Node) {
		count++
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range l.Roots {
		walk(r)
	}
	if count < 30000 {
		t.Errorf("only %d nodes", count)
	}
}
