package scrape

import (
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestChapterTitlesBG(t *testing.T) {
	titles := ChapterTitles(fixture(t, "bg.html"), "bg")
	if got := titles["bg/2"]; got != "Chapter 2: Contents of the Gītā Summarized" {
		t.Errorf("bg/2 = %q", got)
	}
	if len(titles) < 18 {
		t.Errorf("only %d titles", len(titles))
	}
	for r := range titles {
		if strings.Count(r, "/") != 1 {
			t.Errorf("not a direct child: %q", r)
		}
	}
}

func TestChapterTitlesSBAndCC(t *testing.T) {
	for _, c := range []struct{ file, parent string }{{"sb-1.html", "sb/1"}, {"cc-adi.html", "cc/adi"}} {
		titles := ChapterTitles(fixture(t, c.file), c.parent)
		if len(titles) == 0 {
			t.Errorf("%s: no titles", c.file)
		}
		for r := range titles {
			if !strings.HasPrefix(r, c.parent+"/") {
				t.Errorf("%s: key %q", c.file, r)
			}
		}
	}
}

func TestLetterYears(t *testing.T) {
	ys := LetterYears(fixture(t, "letters.html"))
	has := func(y YearCount) bool {
		for _, g := range ys {
			if g == y {
				return true
			}
		}
		return false
	}
	if !has(YearCount{1967, 264}) || !has(YearCount{1969, 760}) {
		t.Errorf("years %v", ys)
	}
	for i := 1; i < len(ys); i++ {
		if ys[i-1].Year >= ys[i].Year {
			t.Fatalf("not sorted: %v", ys)
		}
	}
}

func TestLetterYearPage(t *testing.T) {
	es, next := LetterYearPage(fixture(t, "letters-1967-p1.html"))
	if len(es) != 20 || !next {
		t.Fatalf("%d entries, next=%v", len(es), next)
	}
	if !strings.HasPrefix(es[0].Title, "Letter to:") || !strings.Contains(es[0].Date, "1967") || !strings.HasPrefix(es[0].Ref, "letters/") {
		t.Errorf("first %+v", es[0])
	}
	if es[0].Place != "Vrindaban" {
		t.Errorf("place %q", es[0].Place)
	}
}

func TestGarbageHTML(t *testing.T) {
	g := []byte("<html><body>nope")
	if len(ChapterTitles(g, "bg")) != 0 || len(LetterYears(g)) != 0 {
		t.Error("expected empty")
	}
	if es, next := LetterYearPage(g); len(es) != 0 || next {
		t.Error("expected empty")
	}
}
