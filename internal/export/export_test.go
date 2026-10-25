package export

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vedabase-tui/internal/api"
)

var update = flag.Bool("update", false, "regenerate golden files")

var today = time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)

func load(t *testing.T, name string) api.Item {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var it api.Item
	if err := json.Unmarshal(data, &it); err != nil {
		t.Fatal(err)
	}
	return it
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

var bgMeta = Meta{BookTitle: "Bhagavad-gītā As It Is", Chapter: "2: Contents of the Gītā Summarized"}

func TestItemAllParts(t *testing.T) {
	golden(t, "bg-2-13-all.md", ItemMarkdown(load(t, "verse-bg-2-13.json"), bgMeta, api.AllParts(), today))
}

func TestItemHiddenParts(t *testing.T) {
	p := api.AllParts()
	p.Devanagari, p.Synonyms = false, false
	got := ItemMarkdown(load(t, "verse-bg-2-13.json"), bgMeta, p, today)
	if strings.Contains(got, "देहिनो") || strings.Contains(got, "of the embodied") {
		t.Errorf("hidden parts present:\n%s", got)
	}
	golden(t, "bg-2-13-no-dev-syn.md", got)
}

func TestLetterFrontMatter(t *testing.T) {
	got := ItemMarkdown(load(t, "verse-letter-aatie.json"), Meta{BookTitle: "Letters"}, api.AllParts(), today)
	for _, s := range []string{`date: "October 28th 1967"`, `location: "Navadvipa"`, "year: 1967\n", "# Letter to: Aatie\n"} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q", s)
		}
	}
	golden(t, "letter-aatie.md", got)
}

func TestFrontMatterEscapes(t *testing.T) {
	it := api.Item{Ref: "x/1", Book: "x", URL: "u"}
	got := ItemMarkdown(it, Meta{BookTitle: `A "B" \ C`}, api.AllParts(), today)
	if !strings.Contains(got, `book_title: "A \"B\" \\ C"`) {
		t.Errorf("bad escaping:\n%s", got)
	}
}

func TestCollectionWithMissing(t *testing.T) {
	bg := load(t, "verse-bg-2-13.json")
	got := CollectionMarkdown("On the soul", []Entry{
		{Ref: "bg/2/13", Item: &bg, Meta: bgMeta},
		{Ref: "sb/1/1/1"},
	}, api.AllParts(), today)
	if !strings.Contains(got, "(unavailable: sb/1/1/1)") {
		t.Errorf("missing unavailable marker:\n%s", got)
	}
	golden(t, "collection.md", got)
}

func TestFilenames(t *testing.T) {
	if g := ItemFilename("bg/2/13"); g != "bg-2-13.md" {
		t.Error(g)
	}
	if g := CollectionFilename("On the soul"); g != "collection-on-the-soul.md" {
		t.Error(g)
	}
}

func TestWriteReportsExisted(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")
	p, existed, err := Write(dir, "a.md", "one")
	if err != nil || existed {
		t.Fatalf("first: %v existed=%v", err, existed)
	}
	_, existed, err = Write(dir, "a.md", "two")
	if err != nil || !existed {
		t.Fatalf("second: %v existed=%v", err, existed)
	}
	if b, _ := os.ReadFile(p); string(b) != "two" {
		t.Errorf("content %q", b)
	}
}
