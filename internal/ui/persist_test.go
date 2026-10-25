package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

func lastSaved(t *testing.T, sv *saved) store.State {
	t.Helper()
	sv.mu.Lock()
	defer sv.mu.Unlock()
	if len(sv.calls) == 0 {
		t.Fatal("state never saved")
	}
	return sv.calls[len(sv.calls)-1]
}

// Closing the terminal without q must not lose the reading position.
func TestStateSavedWithoutQuit(t *testing.T) {
	a, sv := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	if got := lastSaved(t, sv); got.LastRef != "bg/2/13" {
		t.Fatalf("after open: %+v", got)
	}
	drive(a, key("l"))
	if got := lastSaved(t, sv); got.LastRef != a.ref || a.ref == "bg/2/13" {
		t.Fatalf("after next item: %+v", got)
	}
	drive(a, key("t"))
	if got := lastSaved(t, sv); !got.TreeHidden {
		t.Fatalf("after t: %+v", got)
	}
	drive(a, key("w"))
	if got := lastSaved(t, sv); !got.Wide {
		t.Fatalf("after w: %+v", got)
	}
	drive(a, key("3"))
	if got := lastSaved(t, sv); got.Parts.Synonyms {
		t.Fatalf("after 3: %+v", got)
	}
}

// A stale render from before a theme switch must not replace the new one.
func TestRenderFromOldThemeDropped(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, renderedMsg{ref: a.ref, width: a.textW(), parts: a.parts, pal: "Some Other Theme", out: "STALEPALETTE"})
	if v := plain(a); strings.Contains(v, "STALEPALETTE") {
		t.Fatalf("render from another theme shown:\n%s", v)
	}
}

func TestAllPartsOffSurvives(t *testing.T) {
	a := New(Options{Backend: newFake(t), State: store.State{Parts: api.AllParts()},
		SaveState: func(store.State) error { return nil }})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		drive(a, key(k))
	}
	st := a.State()
	if st.Parts != (api.Parts{}) {
		t.Fatalf("parts = %+v", st.Parts)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := store.SaveState(path, st); err != nil {
		t.Fatal(err)
	}
	b := New(Options{Backend: newFake(t), State: store.LoadState(path)})
	if b.parts != (api.Parts{}) {
		t.Fatalf("restored parts = %+v, want none", b.parts)
	}
}

// State files written before parts_set existed, with no parts, mean "all on".
func TestOldStateWithoutPartsMeansAll(t *testing.T) {
	a := New(Options{Backend: newFake(t), State: store.State{LastRef: "bg/2/13"}})
	if a.parts != api.AllParts() {
		t.Fatalf("parts = %+v", a.parts)
	}
}
