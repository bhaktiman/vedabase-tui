package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/notes"
	"vedabase-tui/internal/store"
)

var noteNow = time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)

// noteApp boots on bg/2/13 with a notes store in a temp dir and counts the
// state saves (a quit makes one).
func noteApp(t *testing.T, mod func(*api.Item)) (*App, *notes.Store, *saved) {
	t.Helper()
	be := newFake(t)
	if mod != nil {
		it := be.items["bg/2/13"]
		mod(&it)
		be.items["bg/2/13"] = it
	}
	ns, _, err := notes.Open(filepath.Join(t.TempDir(), "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	return noteAppWith(t, be, ns)
}

func noteAppWith(t *testing.T, be *fakeBackend, ns *notes.Store) (*App, *notes.Store, *saved) {
	t.Helper()
	sv := &saved{}
	st := store.State{Parts: api.AllParts(), LastRef: "bg/2/13"}
	a := New(Options{Backend: be, Notes: ns, State: st, SaveState: sv.save, Settings: store.Settings{Mouse: true, ReadingWidth: 60},
		Now: func() time.Time { return noteNow }})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	a.opener = func(string) error { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	return a, ns, sv
}

func noteText(ns *notes.Store, ref string) string {
	n, _ := ns.Get(ref)
	return n.Text
}

func TestAOpensEditorPrefilled(t *testing.T) {
	a, ns, _ := noteApp(t, nil)
	if err := ns.Set("bg/2/13", "old words", noteNow); err != nil {
		t.Fatal(err)
	}
	drive(a, key("a"))
	v := plain(a)
	if !a.noteEd.visible || !strings.Contains(v, "Note · Bg 2.13") || !strings.Contains(v, "old words") {
		t.Fatalf("editor:\n%s", v)
	}
}

func TestNoteSaveKeys(t *testing.T) {
	for _, k := range []string{"esc", "ctrl+s", "ctrl+c"} {
		a, ns, sv := noteApp(t, nil)
		saves := len(sv.calls)
		drive(a, key("a"))
		typeKeys(a, "first")
		drive(a, key("enter"))
		typeKeys(a, "second")
		drive(a, key(k))
		if a.noteEd.visible || noteText(ns, "bg/2/13") != "first\nsecond" {
			t.Errorf("%s: visible=%v note=%q", k, a.noteEd.visible, noteText(ns, "bg/2/13"))
		}
		if toastText(a) != "Note saved" {
			t.Errorf("%s: toast %q", k, toastText(a))
		}
		if len(sv.calls) != saves {
			t.Errorf("%s quit the app", k)
		}
	}
}

func TestNoteEmptyDeletes(t *testing.T) {
	a, ns, _ := noteApp(t, nil)
	ns.Set("bg/2/13", "abc", noteNow)
	drive(a, key("a"), key("backspace"), key("backspace"), key("backspace"), key("esc"))
	if ns.Has("bg/2/13") || toastText(a) != "Note deleted" {
		t.Fatalf("has=%v toast=%q", ns.Has("bg/2/13"), toastText(a))
	}
	// nothing typed, nothing stored: no toast, no file
	clearToast(a)
	drive(a, key("a"), key("esc"))
	if a.noteEd.visible || toastText(a) != "" {
		t.Fatalf("toast %q", toastText(a))
	}
}

func TestNoteSaveErrorKeepsText(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := t.TempDir()
	ns, _, err := notes.Open(filepath.Join(dir, "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	a, _, _ := noteAppWith(t, newFake(t), ns)
	drive(a, key("a"))
	typeKeys(a, "precious")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	for _, k := range []string{"esc", "ctrl+s", "ctrl+c"} {
		drive(a, key(k))
		if !a.noteEd.visible || a.noteEd.ta.Value() != "precious" || !strings.HasPrefix(toastText(a), "Couldn't save: ") {
			t.Fatalf("%s: visible=%v text=%q toast=%q", k, a.noteEd.visible, a.noteEd.ta.Value(), toastText(a))
		}
	}
	os.Chmod(dir, 0o700)
	drive(a, key("esc"))
	if a.noteEd.visible || noteText(ns, "bg/2/13") != "precious" {
		t.Fatalf("after fixing the dir: visible=%v note=%q", a.noteEd.visible, noteText(ns, "bg/2/13"))
	}
}

func TestNoteEditorSwallowsKeys(t *testing.T) {
	a, ns, sv := noteApp(t, nil)
	saves := len(sv.calls)
	drive(a, key("a"))
	want := "q ? / s c R D d J K y a"
	typeKeys(a, want)
	drive(a, key("ctrl+f"))
	if a.help || a.search.visible || a.findBar.visible || a.picker.visible || a.copyMenu.visible || a.colview.visible || a.gotoBar.visible {
		t.Fatal("a typed key fired an action")
	}
	if !a.noteEd.visible || len(sv.calls) != saves {
		t.Fatalf("visible=%v quits=%d", a.noteEd.visible, len(sv.calls)-saves)
	}
	drive(a, key("esc"))
	if got := noteText(ns, "bg/2/13"); got != want {
		t.Fatalf("note = %q", got)
	}
}

func TestNoteEditorTakesPaste(t *testing.T) {
	a, ns, _ := noteApp(t, nil)
	drive(a, key("a"), tea.PasteMsg{Content: "pasted\nlines q?"}, key("esc"))
	if got := noteText(ns, "bg/2/13"); got != "pasted\nlines q?" {
		t.Fatalf("note = %q", got)
	}
}

func TestNoteClickOutsideSaves(t *testing.T) {
	a, ns, _ := noteApp(t, nil)
	drive(a, key("a"))
	typeKeys(a, "kept")
	drive(a, click(0, 0))
	if a.noteEd.visible || noteText(ns, "bg/2/13") != "kept" {
		t.Fatalf("visible=%v note=%q", a.noteEd.visible, noteText(ns, "bg/2/13"))
	}
}

func TestNoteTargetsSidebarLeaf(t *testing.T) {
	a, ns, _ := noteApp(t, nil)
	a.focusTree = true
	for _, c := range a.node.Parent.Children {
		if c.Leaf && c.Ref != a.ref {
			a.tree.Reveal(c)
			break
		}
	}
	n := a.tree.Selected()
	if !a.focusTree || n == nil || !n.Leaf || n.Ref == a.ref {
		t.Fatalf("setup: focus=%v sel=%+v", a.focusTree, n)
	}
	drive(a, key("a"))
	if a.noteEd.ref != n.Ref {
		t.Fatalf("editing %q, want %q", a.noteEd.ref, n.Ref)
	}
	typeKeys(a, "side")
	drive(a, key("esc"))
	if noteText(ns, n.Ref) != "side" || ns.Has("bg/2/13") {
		t.Fatalf("notes = %+v", ns.All())
	}
}

func TestANilStoreToasts(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("a"))
	if a.noteEd.visible || toastText(a) != "Notes are unavailable" {
		t.Fatalf("visible=%v toast=%q", a.noteEd.visible, toastText(a))
	}
}

func TestNoteBlockShownUpdatedRemoved(t *testing.T) {
	a, _, _ := noteApp(t, nil)
	if strings.Contains(plain(a), "Your note") {
		t.Fatal("block shown with no note")
	}
	drive(a, key("a"))
	typeKeys(a, "remember this")
	drive(a, key("esc"))
	if v := plain(a); !strings.Contains(v, "Your note") || !strings.Contains(v, "remember this") {
		t.Fatalf("no block:\n%s", v)
	}
	drive(a, key("a"), key("!"), key("esc"))
	if v := plain(a); !strings.Contains(v, "remember this!") {
		t.Fatalf("not updated:\n%s", v)
	}
	drive(a, key("a"))
	a.noteEd.ta.SetValue("")
	drive(a, key("esc"))
	if v := plain(a); strings.Contains(v, "Your note") || strings.Contains(v, "remember") {
		t.Fatalf("block kept:\n%s", v)
	}
}

func TestNoteIsShownLiterally(t *testing.T) {
	a, _, _ := noteApp(t, nil)
	drive(a, key("a"))
	a.noteEd.ta.SetValue("see [[bg/1/1|Bg 1.1]] and **x**")
	drive(a, key("esc"))
	v := plain(a)
	if !strings.Contains(v, "[[bg/1/1|Bg 1.1]]") || !strings.Contains(v, "**x**") {
		t.Fatalf("note not literal:\n%s", v)
	}
}

func TestStaleRenderWithoutNoteIsDropped(t *testing.T) {
	a, _, _ := noteApp(t, nil)
	drive(a, key("a"))
	typeKeys(a, "fresh")
	drive(a, key("esc"))
	drive(a, renderedMsg{ref: a.ref, lang: a.lang, width: a.textW(), parts: a.parts, pal: a.pal.Name, noteRev: a.noteRev - 1, out: "STALENOTE"})
	if strings.Contains(plain(a), "STALENOTE") {
		t.Fatal("a render from before the note was applied")
	}
}

// linkNoteApp shows an item citing Bg. 4.1 … Bg. 4.n, one per paragraph, with
// a three-line note on it.
func linkNoteApp(t *testing.T, n int) *App {
	t.Helper()
	be := newFake(t)
	data, err := os.ReadFile("../api/testdata/verse-bg-2-13.json")
	if err != nil {
		t.Fatal(err)
	}
	var it api.Item
	if err := json.Unmarshal(data, &it); err != nil {
		t.Fatal(err)
	}
	var tr []string
	it.References = nil
	for i := 1; i <= n; i++ {
		s := "Bg. 4." + string(rune('0'+i))
		tr = append(tr, "See "+s+" here.")
		it.References = append(it.References, api.Reference{Text: s, Ref: "bg/4/" + string(rune('0'+i)), Field: "translation"})
	}
	it.Translation = strings.Join(tr, "\n\n")
	it.Purport = ""
	be.items["bg/2/13"] = it
	ns, _, err := notes.Open(filepath.Join(t.TempDir(), "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ns.Set("bg/2/13", "line one\n\nline [[bg/9/9|three]]\nand four", noteNow); err != nil {
		t.Fatal(err)
	}
	a, _, _ := noteAppWith(t, be, ns)
	drive(a, tea.WindowSizeMsg{Width: 100, Height: 80})
	return a
}

func TestLinksStayRightWithANote(t *testing.T) {
	a := linkNoteApp(t, 3)
	if !strings.Contains(plain(a), "Your note") {
		t.Fatalf("no note block:\n%s", plain(a))
	}
	vis := a.reader.VisibleLinks()
	if len(vis) != 3 {
		t.Fatalf("links = %+v", vis)
	}
	lines := strings.Split(plain(a), "\n") // rows of the screen
	x0, y0 := a.textOrigin()
	for i, l := range vis {
		row := y0 + l.Line - a.reader.Offset()
		rs := []rune(lines[row])
		if got := string(rs[min(x0+l.Start, len(rs)):]); !strings.HasPrefix(got, "Bg. 4."+string(rune('1'+i))) {
			t.Errorf("link %d is drawn at %q", i, got)
		}
	}
	drive(a, key("f"), key("d")) // third label
	if a.ref != "bg/4/3" {
		t.Fatalf("f d opened %q", a.ref)
	}
	a = linkNoteApp(t, 3)
	l := a.reader.VisibleLinks()[1]
	drive(a, click(x0+l.Start+1, y0+l.Line-a.reader.Offset()))
	if a.ref != "bg/4/2" {
		t.Fatalf("click opened %q", a.ref)
	}
}

func TestFindSeesTheNote(t *testing.T) {
	a := linkNoteApp(t, 3)
	drive(a, key("ctrl+f"))
	typeKeys(a, "bg. 4.2")
	drive(a, key("enter"))
	if len(a.reader.matches) != 1 || !strings.Contains(ansiRe.ReplaceAllString(a.reader.lines[a.reader.matches[0].line], ""), "Bg. 4.2") {
		t.Fatalf("matches = %+v", a.reader.matches)
	}
}
