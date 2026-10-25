package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/store"
)

func addTo(t *testing.T, a *App, cs *collections.Store, name string, refs ...string) {
	t.Helper()
	for _, r := range refs {
		it := collections.Item{Ref: r, Book: strings.SplitN(r, "/", 2)[0], Label: "Label of " + r}
		if r == "bg/2/13" {
			it.Label = "Bg 2.13 — According to the three modes"
		}
		if _, err := cs.Add(name, it); err != nil {
			t.Fatal(err)
		}
	}
	a.syncCollections()
}

func TestSidebarShowsCollections(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	addTo(t, a, cs, "On the soul", "bg/2/13")
	a.tree.offset = 0
	v := ansiRe.ReplaceAllString(a.tree.View(), "")
	lines := strings.Split(v, "\n")
	for i, want := range []string{"★ Collections", "On the soul  1", "Bg 2.13"} {
		if i >= len(lines) || !strings.Contains(lines[i], want) {
			t.Fatalf("row %d lacks %q:\n%s", i, want, v)
		}
	}
}

func TestSidebarUpdatesAfterAdd(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if slices.Contains(labels(a.tree), "Mix") {
		t.Fatal("collection before it exists")
	}
	drive(a, key("s"), key("enter"))
	typeKeys(a, "Mix")
	drive(a, key("enter"))
	ls := labels(a.tree)
	if !slices.Contains(ls, "Mix") || !strings.HasPrefix(ls[slices.Index(ls, "Mix")+1], "Bg 2.13") {
		t.Fatalf("rows = %v", ls)
	}
	if slices.Contains(ls, "Press s on a verse to start one") {
		t.Fatal("hint still shown")
	}
}

func TestSidebarEmptyHint(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.tree.offset = 0
	if v := plain(a); !strings.Contains(v, "Press s on a verse to start one") {
		t.Fatalf("no hint:\n%s", v)
	}
	// Enter and a click on the hint do nothing.
	a.focusTree = true
	a.tree.move(1)
	before := a.ref
	drive(a, key("enter"), key("l"))
	tr, cmd := a.tree.Click(1, false)
	a.tree = tr
	if cmd != nil || a.ref != before || !a.tree.expanded["collections#"] {
		t.Fatalf("hint reacted: cmd=%v ref=%q", cmd, a.ref)
	}
}

func TestCollectionReadingOrder(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "sb/1/1/1"})
	addTo(t, a, cs, "Mix", "bg/2/13", "sb/1/1/1")
	a.focusTree = true
	a.tree.Reveal(a.lib.CollectionItem("Mix", "bg/2/13"))
	drive(a, key("enter"))
	if a.ref != "bg/2/13" {
		t.Fatalf("ref = %q", a.ref)
	}
	if bar := ansiRe.ReplaceAllString(a.topBar(), ""); !strings.Contains(bar, "★ Collections › Mix › Bg 2.13") || !strings.Contains(bar, "1 / 2") {
		t.Fatalf("top bar = %q", bar)
	}
	drive(a, key("l"))
	if a.ref != "sb/1/1/1" {
		t.Fatalf("l went to %q, want the next item of the collection", a.ref)
	}
	clearToast(a)
	drive(a, key("l"))
	if toastText(a) != "End of Mix" || a.ref != "sb/1/1/1" {
		t.Fatalf("toast %q ref %q", toastText(a), a.ref)
	}
	drive(a, key("h"))
	clearToast(a)
	drive(a, key("h"))
	if toastText(a) != "Start of Mix" || a.ref != "bg/2/13" {
		t.Fatalf("toast %q ref %q", toastText(a), a.ref)
	}
	// The same verse from the book tree keeps book order.
	drive(a, a.Open("bg/2/13")())
	drive(a, key("l"))
	if want := a.lib.Next(a.lib.Find("bg/2/13")).Ref; a.ref != want || want == "sb/1/1/1" {
		t.Fatalf("book order broken: %q", a.ref)
	}
}

func TestExportFromSidebar(t *testing.T) {
	a, cs, dir := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	addTo(t, a, cs, "Mix", "bg/2/13", "bg/2/14")
	a.focusTree = true
	a.tree.Reveal(a.lib.Roots[0].Children[0])
	drive(a, key("e"))
	if _, err := os.Stat(filepath.Join(dir, "collection-mix.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSidebarSurvivesLibraryRefresh(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	addTo(t, a, cs, "Mix", "bg/2/13")
	a.focusTree = true
	a.tree.Reveal(a.lib.CollectionItem("Mix", "bg/2/13"))
	drive(a, key("enter"))
	drive(a, libraryMsg{lib: sampleLibrary(t)})
	if !slices.Contains(labels(a.tree), "Mix") {
		t.Fatalf("rows = %v", labels(a.tree))
	}
	if _, ok := inColl(a); !ok {
		t.Fatal("reader lost its collection")
	}
}

func inColl(a *App) (string, bool) {
	if a.node == nil || a.node.Parent == nil || a.node.Parent.Parent == nil {
		return "", false
	}
	return a.node.Parent.Seg, a.node.Parent.Parent.Ref == "collections#"
}

func TestCollapseAll(t *testing.T) {
	for _, treeFocus := range []bool{true, false} {
		a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
		a.tree.Reveal(a.lib.Find("letters#to").Children[0])
		a.tree.Reveal(a.lib.Find("bg/2/13"))
		if a.tree.Selected().Ref != "bg/2/13" || !a.tree.expanded["letters#to"] {
			t.Fatal("setup")
		}
		a.focusTree = treeFocus
		drive(a, key("z"))
		var want []string
		for _, r := range a.lib.Roots {
			want = append(want, r.Label())
		}
		if got := labels(a.tree); !slices.Equal(got, want) {
			t.Fatalf("focus=%v rows = %v, want %v", treeFocus, got, want)
		}
		if got := a.tree.Selected().Label(); got != "Bhagavad-gītā As It Is" {
			t.Fatalf("cursor on %q", got)
		}
	}
}

func TestAddKeepsTreeCursorInBook(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	addTo(t, a, cs, "On the soul", "bg/2/1")
	a.focusTree = true
	a.tree.Reveal(a.lib.Find("bg/2/13"))
	row := a.tree.cursor
	drive(a, key("s"), key("j"), key("enter"))
	typeKeys(a, "Other")
	drive(a, key("enter"))
	n := a.tree.Selected()
	if n == nil || n.Ref != "bg/2/13" || n.Parent.Ref != "bg/2" {
		t.Fatalf("cursor on %+v", n)
	}
	if d := a.tree.cursor - row; d < 0 || d > 3 {
		t.Fatalf("cursor moved from %d to %d", row, a.tree.cursor)
	}
}

func TestRebuildKeepsCursorOnSecondCollectionCopy(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	addTo(t, a, cs, "A", "bg/2/13")
	addTo(t, a, cs, "B", "bg/2/13")
	a.focusTree = true
	a.tree.Reveal(a.lib.CollectionItem("B", "bg/2/13"))
	a.tree.SetLibrary(a.lib)
	addTo(t, a, cs, "C", "bg/2/1")
	n := a.tree.Selected()
	if name, _ := library.InCollection(n); n.Ref != "bg/2/13" || name != "B" {
		t.Fatalf("cursor on %q in %q", n.Ref, name)
	}
}
