package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/store"
)

func colApp(t *testing.T, be *fakeBackend, st store.State) (*App, *collections.Store, string) {
	t.Helper()
	cs, _, err := collections.Open(filepath.Join(t.TempDir(), "collections.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "exports")
	if st.Parts == (api.Parts{}) {
		st.Parts = api.AllParts()
	}
	a := New(Options{Backend: be, Collections: cs, State: st, Settings: store.Settings{ExportDir: dir},
		Now: func() time.Time { return time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) }})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	return a, cs, dir
}

func toastText(a *App) string {
	if a.toasts.cur == nil {
		return ""
	}
	return a.toasts.cur.text
}

func clearToast(a *App) { a.toasts.cur, a.toasts.queue = nil, nil }

func TestStarAddsToNewCollection(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("s"), key("enter")) // only "+ New collection…" exists
	typeKeys(a, "On the soul")
	drive(a, key("enter"))
	col, ok := cs.Get("On the soul")
	if !ok || len(col.Items) != 1 || col.Items[0].Ref != "bg/2/13" || col.Items[0].Book != "bg" {
		t.Fatalf("store = %+v", cs.List())
	}
	if l := col.Items[0].Label; !strings.HasPrefix(l, "Bg 2.13 — ") || strings.Contains(l, "**") {
		t.Fatalf("label = %q", l)
	}
	if got := toastText(a); got != "★ Added to On the soul" {
		t.Fatalf("toast = %q", got)
	}
	if !strings.Contains(plain(a), "★") {
		t.Fatalf("no star in tree:\n%s", plain(a))
	}
}

func TestStarDuplicateToast(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("s"), key("enter"))
	typeKeys(a, "On the soul")
	drive(a, key("enter"))
	clearToast(a)
	drive(a, key("s"), key("enter"))
	if got := toastText(a); got != "Already in On the soul" {
		t.Fatalf("toast = %q", got)
	}
}

func seeded(t *testing.T) (*App, *collections.Store, *fakeBackend, string) {
	be := newFake(t)
	a, cs, dir := colApp(t, be, store.State{LastRef: "bg/2/14"})
	for _, r := range []string{"bg/2/13", "bg/2/1"} {
		if _, err := cs.Add("Soul", collections.Item{Ref: r, Book: "bg", Label: "Label " + r}); err != nil {
			t.Fatal(err)
		}
	}
	return a, cs, be, dir
}

func TestCollectionsViewOpenItem(t *testing.T) {
	a, _, _, _ := seeded(t)
	openColView(a)
	if v := plain(a); !strings.Contains(v, "Soul") || !strings.Contains(v, "2") {
		t.Fatalf("view:\n%s", v)
	}
	drive(a, key("enter"), key("enter"))
	if a.ref != "bg/2/13" || a.colview.visible {
		t.Fatalf("ref = %q visible=%v", a.ref, a.colview.visible)
	}
}

func TestRemoveRenameDelete(t *testing.T) {
	a, cs, _, _ := seeded(t)
	openColView(a)
	drive(a, key("enter"), key("d"), key("y"))
	if col, _ := cs.Get("Soul"); len(col.Items) != 1 || col.Items[0].Ref != "bg/2/1" {
		t.Fatalf("items = %+v", col.Items)
	}
	drive(a, key("esc"), key("R"))
	for i := 0; i < 4; i++ {
		drive(a, tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typeKeys(a, "Self")
	drive(a, key("enter"))
	if _, ok := cs.Get("Self"); !ok || len(cs.List()) != 1 {
		t.Fatalf("list = %+v", cs.List())
	}
	drive(a, key("D"), key("n"))
	if len(cs.List()) != 1 {
		t.Fatal("D,n deleted")
	}
	drive(a, key("D"), key("y"))
	if len(cs.List()) != 0 {
		t.Fatal("D,y kept")
	}
}

func TestExportItem(t *testing.T) {
	a, _, dir := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("3")) // hide synonyms
	drive(a, key("e"))
	b, err := os.ReadFile(filepath.Join(dir, "bg-2-13.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "dehinaḥ") || !strings.Contains(s, "chapter:") || !strings.Contains(s, `exported: "2026-10-25"`) {
		t.Fatalf("file:\n%s", s)
	}
	if !strings.HasPrefix(toastText(a), "✓ Exported ") {
		t.Fatalf("toast = %q", toastText(a))
	}
	clearToast(a)
	drive(a, key("e"))
	if !strings.HasPrefix(toastText(a), "✓ Overwrote ") {
		t.Fatalf("toast = %q", toastText(a))
	}
}

func TestExportCollectionFetchesMissing(t *testing.T) {
	be := newFake(t)
	a, cs, dir := colApp(t, be, store.State{LastRef: "bg/2/13"})
	for _, r := range []string{"bg/2/13", "bg/2/1", "sb/1/1/1"} {
		if _, err := cs.Add("Mix", collections.Item{Ref: r, Book: strings.SplitN(r, "/", 2)[0], Label: r}); err != nil {
			t.Fatal(err)
		}
	}
	be.cached["bg/2/13"] = true
	be.itemErr["sb/1/1/1"] = os.ErrNotExist
	be.itemCall = nil
	openColView(a)
	drive(a, key("e"))
	if !be.called("bg/2/1") || !be.called("sb/1/1/1") {
		t.Fatalf("calls = %v", be.itemCall)
	}
	b, err := os.ReadFile(filepath.Join(dir, "collection-mix.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"dehinaḥ", "Translation of bg/2/1", "(unavailable: sb/1/1/1)"} {
		if !strings.Contains(s, want) {
			t.Errorf("file lacks %q", want)
		}
	}
	if !strings.Contains(toastText(a), "1 unavailable") {
		t.Fatalf("toast = %q", toastText(a))
	}
	if a.exporting {
		t.Fatal("still exporting")
	}
}

func TestSecondExportWhileRunning(t *testing.T) {
	be := newFake(t)
	a, cs, _ := colApp(t, be, store.State{LastRef: "bg/2/13"})
	if _, err := cs.Add("Mix", collections.Item{Ref: "bg/2/1", Book: "bg"}); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	be.block["bg/2/1"] = gate
	_, cmd := a.Update(exportCollectionMsg{"Mix"})
	if cmd == nil || !a.exporting || !strings.Contains(a.exportStatus(), "Exporting Mix 0/1") {
		t.Fatal("not started")
	}
	_, _ = a.Update(exportCollectionMsg{"Mix"})
	if toastText(a) != "An export is already running" {
		t.Fatalf("toast = %q", toastText(a))
	}
	close(gate)
	drive(a, cmd())
	if a.exporting {
		t.Fatal("still exporting")
	}
}

// The picker lists the most recently used collection first; Tab switches to A–Z.
func TestPickerOrder(t *testing.T) {
	a, cs, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		name string
		d    int
	}{{"Alpha", 1}, {"Beta", 5}, {"Gamma", 3}, {"Alpha", 4}} { // Alpha used again on day 4
		if _, err := cs.Add(c.name, collections.Item{Ref: "bg/1/" + strconv.Itoa(c.d), Added: day(c.d)}); err != nil {
			t.Fatal(err)
		}
	}
	drive(a, key("s"))
	if want := []string{"Beta", "Alpha", "Gamma"}; !slices.Equal(a.picker.names, want) {
		t.Fatalf("recent order %v, want %v", a.picker.names, want)
	}
	if !strings.Contains(plain(a), "recent first (tab: A–Z)") {
		t.Fatal("no sort hint")
	}
	drive(a, key("j"), key("tab")) // on Alpha
	if want := []string{"Alpha", "Beta", "Gamma"}; !slices.Equal(a.picker.names, want) || a.picker.cursor != 0 {
		t.Fatalf("A–Z order %v cursor %d", a.picker.names, a.picker.cursor)
	}
	if !strings.Contains(plain(a), "A–Z (tab: recent first)") {
		t.Fatal("no hint back to recent")
	}
	drive(a, key("esc"), key("s")) // the choice holds while the app runs
	if a.picker.names[0] != "Alpha" {
		t.Fatalf("reopened %v", a.picker.names)
	}
	drive(a, key("tab"), key("esc"), key("s"), key("enter")) // recent again; Enter adds to the most recent
	if c, _ := cs.Get("Beta"); len(c.Items) != 2 {
		t.Fatalf("Beta has %d items", len(c.Items))
	}
}

func jumpApp(t *testing.T, st store.State) (*App, *collections.Store) {
	t.Helper()
	a, cs, _ := colApp(t, newFake(t), st)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		name, ref string
		d         int
	}{{"Alpha", "bg/1/1", 1}, {"Beta", "bg/2/13", 5}, {"Beta", "bg/2/1", 6}} {
		if _, err := cs.Add(c.name, collections.Item{Ref: c.ref, Book: "bg", Label: "L " + c.ref, Added: day(c.d)}); err != nil {
			t.Fatal(err)
		}
	}
	drive(a, key("l")) // let syncCollections run; harmless
	return a, cs
}

func TestJumpPickerListsAndOrders(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	drive(a, key("c"))
	v := plain(a)
	for _, w := range []string{"Go to collection", "Beta  ·  2", "Alpha  ·  1", "enter open", "recent first (tab: A–Z)"} {
		if !strings.Contains(v, w) {
			t.Errorf("view lacks %q:\n%s", w, v)
		}
	}
	if strings.Contains(v, newCollectionRow) {
		t.Error("jump picker has a new-collection row")
	}
	if !slices.Equal(a.picker.names, []string{"Beta", "Alpha"}) {
		t.Fatalf("order %v", a.picker.names)
	}
	drive(a, key("tab"))
	if !slices.Equal(a.picker.names, []string{"Alpha", "Beta"}) {
		t.Fatalf("A–Z order %v", a.picker.names)
	}
	// Rows: Alpha, its item, Beta, its two items. The cursor stops at the
	// last row, not on a phantom one.
	drive(a, key("j"), key("j"), key("j"), key("j"), key("j"), key("j"))
	if a.picker.cursor != 4 {
		t.Fatalf("cursor %d", a.picker.cursor)
	}
	a.picker.Wheel(5)
	if a.picker.cursor != 4 {
		t.Fatalf("wheel cursor %d", a.picker.cursor)
	}
	// A normal `s` picker afterwards still has its extra row.
	drive(a, key("esc"), key("s"))
	if a.picker.jump || !strings.Contains(plain(a), newCollectionRow) {
		t.Fatal("s picker lost its new row")
	}
}

func TestJumpPickerClick(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	drive(a, key("c"))
	a.picker.Click(5, 3) // second row selects
	if a.picker.cursor != 1 {
		t.Fatalf("cursor %d", a.picker.cursor)
	}
	if cmd := a.picker.Click(5, 7); cmd != nil { // below the last row
		t.Fatal("clicked a phantom row")
	}
	drive(a, a.picker.Click(5, 3))
	if a.picker.visible {
		t.Fatal("click on the selected row did not choose")
	}
}

func TestJumpToastsWhenEmptyOrUnavailable(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("c"))
	if got := toastText(a); got != "No collections yet — press s while reading" || a.picker.visible {
		t.Fatalf("toast %q visible %v", got, a.picker.visible)
	}
	b, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	b.cols = nil
	drive(b, key("c"))
	if got := toastText(b); got != "Collections are unavailable" {
		t.Fatalf("toast %q", got)
	}
}

func TestJumpRevealsInSidebar(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	a.focusTree = false
	drive(a, key("c"), key("j"), key("j"), key("j"), key("enter")) // Alpha
	n := a.tree.Selected()
	if n == nil || n.Seg != "Alpha" || n.Parent == nil || n.Parent.Ref != library.CollectionsRef {
		t.Fatalf("selected %+v", n)
	}
	if !a.focusTree || a.picker.visible || a.colview.visible {
		t.Fatalf("focusTree %v picker %v colview %v", a.focusTree, a.picker.visible, a.colview.visible)
	}
	if !strings.Contains(plain(a), "L bg/1/1") {
		t.Fatalf("collection not expanded:\n%s", plain(a))
	}
}

func TestJumpClearsHidingFilter(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	a.tree.SetFilter("zzzz-nothing")
	drive(a, key("c"), key("enter")) // Beta
	if n := a.tree.Selected(); n == nil || n.Seg != "Beta" {
		t.Fatalf("selected %+v", n)
	}
}

func TestJumpWithoutSidebarOpensOverlayInside(t *testing.T) {
	for name, st := range map[string]store.State{
		"hidden": {LastRef: "bg/2/13", TreeHidden: true},
		"zen":    {LastRef: "bg/2/13", Zen: true},
	} {
		a, _ := jumpApp(t, st)
		drive(a, key("c"), key("j"), key("j"), key("j"), key("enter")) // Alpha
		if !a.colview.visible || a.colview.in != "Alpha" || a.picker.visible {
			t.Fatalf("%s: colview %v in %q", name, a.colview.visible, a.colview.in)
		}
		if !strings.Contains(plain(a), "L bg/1/1") {
			t.Fatalf("%s: items not shown", name)
		}
		drive(a, key("esc")) // back at the list, on Alpha (index 0 of A–Z list)
		if a.colview.in != "" || !a.colview.visible || a.colview.cursor != 0 {
			t.Fatalf("%s: after esc in=%q cursor=%d", name, a.colview.in, a.colview.cursor)
		}
	}
	// Too narrow for a sidebar.
	a, _ := jumpApp(t, store.State{LastRef: "bg/2/13"})
	drive(a, tea.WindowSizeMsg{Width: minTreeCols - 1, Height: 40}, key("c"), key("enter"))
	if !a.colview.visible || a.colview.in != "Beta" {
		t.Fatalf("narrow: colview %v in %q", a.colview.visible, a.colview.in)
	}
}

func TestHelpHasCollectionsGroupInTwoColumns(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("?"))
	v := plain(a)
	for _, w := range []string{"Collections", "Jump to collection / item", "Rename collection", "remove item", "Move item down / up", "Export collection"} {
		if !strings.Contains(v, w) {
			t.Errorf("help lacks %q", w)
		}
	}
	lines := strings.Split(v, "\n")
	if len(lines) > 40 {
		t.Errorf("help is %d rows", len(lines))
	}
	for _, l := range lines {
		if w := lipgloss.Width(l); w > 120 {
			t.Fatalf("line %d wide: %q", w, l)
		}
	}
	if h := lipgloss.Height(a.helpView()); h > 30 { // fits a 120×30 terminal
		t.Errorf("two-column help is %d rows tall", h)
	}
	if strings.Contains(v, "Open collections") {
		t.Error("stale Overlays rows")
	}
}

// The `c` picker lists each collection's items under it; Enter on an item
// opens it as the sidebar would, inside its collection.
func TestJumpPickerListsAndOpensItems(t *testing.T) {
	a, _ := jumpApp(t, store.State{LastRef: "bg/1/1"})
	drive(a, key("c"))
	want := []jumpRow{{"Beta", ""}, {"Beta", "bg/2/13"}, {"Beta", "bg/2/1"}, {"Alpha", ""}, {"Alpha", "bg/1/1"}}
	if !slices.Equal(a.picker.rows, want) {
		t.Fatalf("rows %v", a.picker.rows)
	}
	v := plain(a)
	for _, w := range []string{"Beta  ·  2", "    L bg/2/13  ·  bg", "enter open"} {
		if !strings.Contains(v, w) {
			t.Errorf("view lacks %q:\n%s", w, v)
		}
	}
	drive(a, key("j"), key("j"), key("enter")) // Beta's bg/2/1
	if a.picker.visible || a.colview.visible {
		t.Fatal("overlay still open")
	}
	if a.ref != "bg/2/1" || a.focusTree {
		t.Fatalf("ref %q focusTree %v", a.ref, a.focusTree)
	}
	if name, ok := library.InCollection(a.node); !ok || name != "Beta" {
		t.Fatalf("node %+v not under Beta", a.node)
	}
	if n := a.tree.Selected(); n != a.node {
		t.Fatalf("sidebar cursor on %+v", n)
	}
	// Without a sidebar it still opens the item, no overlay.
	b, _ := jumpApp(t, store.State{LastRef: "bg/1/1", Zen: true})
	drive(b, key("c"), key("j"), key("enter"))
	if b.ref != "bg/2/13" || b.colview.visible || b.picker.visible {
		t.Fatalf("zen: ref %q colview %v", b.ref, b.colview.visible)
	}
}

// openColView opens the collections overlay the way R / D and c (sidebar
// hidden) do; it has no key of its own.
func openColView(a *App) { a.colview.Show(a.cols) }

// ' no longer opens anything: c covers jumping, the sidebar covers managing.
func TestApostropheOpensNothing(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	before := plain(a)
	drive(a, key("'"))
	if a.colview.visible || a.picker.visible || plain(a) != before {
		t.Fatal("' still does something")
	}
}
