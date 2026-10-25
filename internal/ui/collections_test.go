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
	drive(a, key("'"))
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
	drive(a, key("'"), key("enter"), key("d"), key("y"))
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
	drive(a, key("'"), key("e"))
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
