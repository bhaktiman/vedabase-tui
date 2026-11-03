package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/notes"
	"vedabase-tui/internal/store"
)

// markApp is an app on bg/2/13 with both a collections and a notes store;
// "Mix" holds bg/2/13 and bg/2/14.
func markApp(t *testing.T) (*App, *notes.Store, string) {
	t.Helper()
	cs, _, err := collections.Open(filepath.Join(t.TempDir(), "collections.json"))
	if err != nil {
		t.Fatal(err)
	}
	ns, _, err := notes.Open(filepath.Join(t.TempDir(), "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "exports")
	a := New(Options{Backend: newFake(t), Collections: cs, Notes: ns,
		State:    store.State{Parts: api.AllParts(), LastRef: "bg/2/13"},
		Settings: store.Settings{ExportDir: dir},
		Now:      func() time.Time { return noteNow }})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	a.opener = func(string) error { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	addTo(t, a, cs, "Mix", "bg/2/13", "bg/2/14")
	return a, ns, dir
}

func sidebarText(a *App) string {
	a.tree.offset = 0
	return ansiRe.ReplaceAllString(a.tree.View(), "")
}

func TestMarkerFollowsNoteInSidebar(t *testing.T) {
	a, _, _ := markApp(t)
	if strings.Contains(sidebarText(a), "✎") {
		t.Fatalf("marker without a note:\n%s", sidebarText(a))
	}
	drive(a, key("a"))
	a.noteEd.ta.SetValue("remember this")
	drive(a, key("esc"))
	v := sidebarText(a)
	if strings.Count(v, "✎") != 1 || !strings.Contains(v, "Bg 2.13 — According to th… ✎") {
		t.Fatalf("row lacks the marker, or another row has it:\n%s", v)
	}
	drive(a, key("a"))
	a.noteEd.ta.SetValue("")
	drive(a, key("esc"))
	if strings.Contains(sidebarText(a), "✎") {
		t.Fatalf("marker after the note was deleted:\n%s", sidebarText(a))
	}
}

func TestMarkerInJumpPickerAndList(t *testing.T) {
	a, ns, _ := markApp(t)
	if err := ns.Set("bg/2/13", "n", noteNow); err != nil {
		t.Fatal(err)
	}
	drive(a, key("c"))
	v := plain(a)
	if strings.Count(v, "✎  ·  bg") != 1 || strings.Count(v, "✎") != 2 {
		t.Fatalf("jump picker markers:\n%s", v)
	}
	drive(a, key("esc"))
	openColView(a)
	drive(a, key("enter"))
	v = plain(a)
	if strings.Count(v, "three modes ✎") != 1 || strings.Count(v, "✎") != 2 {
		t.Fatalf("collection list markers:\n%s", v)
	}
}

func TestExportedItemAndCollectionHaveNote(t *testing.T) {
	a, ns, dir := markApp(t)
	if err := ns.Set("bg/2/13", "line one\nline two", noteNow); err != nil {
		t.Fatal(err)
	}
	drive(a, key("e"))
	b, err := os.ReadFile(filepath.Join(dir, "bg-2-13.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\n> **Note:** line one\n> line two\n") {
		t.Fatalf("item file:\n%s", b)
	}
	a.focusTree = true
	a.tree.Reveal(a.lib.Roots[0].Children[0])
	drive(a, key("e"))
	// the export runs on a goroutine; wait for its file
	var c []byte
	for i := 0; i < 200; i++ {
		if c, err = os.ReadFile(filepath.Join(dir, "collection-mix.md")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || strings.Count(string(c), "Note:") != 1 || !strings.Contains(string(c), "> line two") {
		t.Fatalf("collection file (%v):\n%s", err, c)
	}
}

func TestCopyEverythingHasNote(t *testing.T) {
	a, ns, _ := markApp(t)
	if err := ns.Set("bg/2/13", "my note", noteNow); err != nil {
		t.Fatal(err)
	}
	a.openCopyMenu()
	for _, r := range a.copyMenu.rows {
		if r.label == "Everything" {
			if !strings.Contains(r.text, "> **Note:** my note") {
				t.Fatalf("Everything lacks the note:\n%s", r.text)
			}
			return
		}
	}
	t.Fatal("no Everything row")
}

// A [[x|y]] the user typed in a note is copied as typed; the item's own
// links still become labels.
func TestCopyEverythingKeepsNoteLiteral(t *testing.T) {
	a, ns, _ := markApp(t)
	if err := ns.Set("bg/2/13", "see [[bg/1/1|Bg 1.1]]", noteNow); err != nil {
		t.Fatal(err)
	}
	a.item.Translation = "As in [[bg/2/12|Bg 2.12]]."
	a.openCopyMenu()
	for _, r := range a.copyMenu.rows {
		if r.label == "Everything" {
			if !strings.Contains(r.text, "see [[bg/1/1|Bg 1.1]]") || !strings.Contains(r.text, "As in Bg 2.12.") {
				t.Fatalf("Everything:\n%s", r.text)
			}
			return
		}
	}
	t.Fatal("no Everything row")
}
