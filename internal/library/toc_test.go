package library

import (
	"encoding/json"
	"os"
	"testing"

	"vedabase-tui/internal/api"
)

func readTOC(t *testing.T, name string) api.TOC {
	t.Helper()
	b, err := os.ReadFile("../api/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var toc api.TOC
	if err := json.Unmarshal(b, &toc); err != nil {
		t.Fatal(err)
	}
	return toc
}

func leafRefs(l *Library, book string) []string {
	return refs(l.flat[book])
}

func tocLeafRefs(ns []api.TOCNode) []string {
	var out []string
	for _, n := range ns {
		if len(n.Children) == 0 {
			out = append(out, n.Ref)
		}
		out = append(out, tocLeafRefs(n.Children)...)
	}
	return out
}

func TestApplyTOCBg(t *testing.T) {
	l := load(t)
	oldNode := l.Find("bg/2/13")
	toc := readTOC(t, "toc-bg.json")
	if !l.ApplyTOC("bg", toc) {
		t.Fatal("ApplyTOC = false")
	}
	if l.Find("bg/2/13") == oldNode {
		t.Error("old node survived")
	}
	eq(t, leafRefs(l, "bg"), tocLeafRefs(toc.Children))
	eq(t, refs(l.Find("bg").Children), refs2(toc.Children))

	n := l.Find("bg/1/16-18")
	if n == nil || !n.Leaf || n.Parent.Ref != "bg/1" {
		t.Fatalf("bg/1/16-18 = %+v", n)
	}
	if got := n.Label(); got != "1.16-18" {
		t.Errorf("leaf label = %q", got)
	}
	if got := l.Find("bg/1").Label(); got != "Chapter 1: Observing the Armies on the Battlefield of Kurukṣetra" {
		t.Errorf("chapter label = %q", got)
	}
	if got := l.Find("bg/0a"); got == nil || !got.Leaf || got.Label() == "" {
		t.Errorf("front matter = %+v", got)
	}
	if got := l.Find("bg").Label(); got != "BG title" {
		t.Errorf("root label changed: %q", got)
	}
	if i, total := Position(n); i != 16 || total != 39 {
		t.Errorf("Position = %d,%d", i, total)
	}
	last := toc.Children[4].Children
	next := l.Next(l.Find(last[len(last)-1].Ref))
	if next == nil || next.Ref != toc.Children[5].Children[0].Ref {
		t.Errorf("next across chapters = %v", next)
	}
	if p := l.Prev(next); p == nil || p.Ref != last[len(last)-1].Ref {
		t.Errorf("prev = %v", p)
	}
}

func refs2(ns []api.TOCNode) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Ref)
	}
	return out
}

func TestApplyTOCSbKeepsOrderNotSorted(t *testing.T) {
	l := load(t)
	toc := readTOC(t, "toc-sb-trim.json")
	if !l.ApplyTOC("sb", toc) {
		t.Fatal("false")
	}
	eq(t, refs(l.Find("sb").Children), []string{"sb/0a", "sb/0b", "sb/1"})
	eq(t, leafRefs(l, "sb"), []string{
		"sb/0a", "sb/0b", "sb/1/1/1", "sb/1/1/2", "sb/1/1/3", "sb/1/2/1", "sb/1/2/2", "sb/1/2/3"})
	if got := l.Find("sb/0a").Label(); got != "Preface" {
		t.Errorf("front matter = %q", got)
	}
	if got := l.Find("sb/1").Label(); got != "Canto 1: Creation" {
		t.Errorf("canto = %q", got)
	}
	if got := l.Find("sb/1/1/2").Label(); got != "1.1.2" {
		t.Errorf("leaf = %q", got)
	}
	if got := l.Next(l.Find("sb/1/1/3")).Ref; got != "sb/1/2/1" {
		t.Errorf("next = %s", got)
	}
	if l.Find("sb/4/29/1b") != nil {
		t.Error("sitemap-only node survived")
	}
}

func TestApplyTOCCc(t *testing.T) {
	l := load(t)
	if !l.ApplyTOC("cc", readTOC(t, "toc-cc-trim.json")) {
		t.Fatal("false")
	}
	if got := l.Find("cc/adi").Label(); got != "Ādi-līlā" {
		t.Errorf("lila = %q", got)
	}
	if got := l.Find("cc/adi/1/2").Label(); got != "1.2" {
		t.Errorf("leaf = %q", got)
	}
	if got := l.Find("cc/adi/0a").Label(); got != "Preface" {
		t.Errorf("preface = %q", got)
	}
	eq(t, leafRefs(l, "cc"), []string{"cc/adi/0a", "cc/adi/1/1", "cc/adi/1/2", "cc/adi/1/3"})
}

func TestApplyTOCFlatBook(t *testing.T) {
	l := load(t)
	toc := readTOC(t, "toc-iso.json")
	if !l.ApplyTOC("iso", toc) {
		t.Fatal("false")
	}
	eq(t, leafRefs(l, "iso"), refs2(toc.Children))
	if got := l.Find("iso/1").Label(); got != "Chapter 1: Mantra 1" {
		t.Errorf("label = %q", got)
	}
	if got := l.Find("iso/0a").Label(); got != "Front matter (0a)" {
		t.Errorf("untitled front matter = %q", got)
	}
}

func TestApplyTOCRefusals(t *testing.T) {
	l := load(t)
	toc := readTOC(t, "toc-iso.json")
	for _, code := range []string{"nope", "letters", "transcripts", "collections"} {
		if l.ApplyTOC(code, toc) {
			t.Errorf("ApplyTOC(%q) = true", code)
		}
	}
	if l.ApplyTOC("bg", api.TOC{}) {
		t.Error("empty TOC replaced the fallback tree")
	}
	if l.Find("bg/2/13") == nil {
		t.Error("fallback tree lost")
	}
	if l.Find("letters#year") == nil || len(l.Find("letters#to").Children) == 0 {
		t.Error("letters touched")
	}
}

func TestApplyTOCSkipsNodesWithoutRef(t *testing.T) {
	l := load(t)
	toc := api.TOC{Book: "bg", Children: []api.TOCNode{
		{Ref: "bg/1", Title: "One", Children: []api.TOCNode{{Ref: "bg/1/1", Label: "1.1"}}},
		{Ref: "", Title: "Ghost", Children: []api.TOCNode{{Ref: "bg/2/1", Label: "2.1"}}},
		{Ref: "sb/9", Label: "alien"},
	}}
	if !l.ApplyTOC("bg", toc) {
		t.Fatal("false")
	}
	eq(t, leafRefs(l, "bg"), []string{"bg/1/1", "bg/2/1"})
	if l.Find("sb/9") != nil {
		t.Error("foreign ref indexed")
	}
}

func TestApplyTOCTwiceIsStable(t *testing.T) {
	l := load(t)
	toc := readTOC(t, "toc-bg.json")
	l.ApplyTOC("bg", toc)
	l.ApplyTOC("bg", toc)
	eq(t, leafRefs(l, "bg"), tocLeafRefs(toc.Children))
	if len(l.flatAt) < len(l.flat["bg"]) {
		t.Error("flatAt incomplete")
	}
}
