package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	vedabasetui "vedabase-tui"
	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

const testLog = `# What's new

## v0.3.3 — 2026-11-03

- **Copy what you tick.** ` + "`y`" + ` opens tick boxes.
- **Pasting works.**

## v0.3.2 — 2026-11-03

- **Notes.** a writes a note.

## v0.3.1 — 2026-10-27

- **Items refresh.**

## v0.2.1 — 2026-10-26

- **Languages fixed.**

## v0.2.0 — 2026-10-26

- **Clickable references.**
`

func bootVersion(t *testing.T, st store.State, version, log string) (*App, *saved) {
	t.Helper()
	sv := &saved{}
	if st.Parts == (api.Parts{}) {
		st.Parts = api.AllParts()
	}
	a := New(Options{Backend: newFake(t), State: st, SaveState: sv.save, Version: version, Changelog: log})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	return a, sv
}

func TestChangelogSince(t *testing.T) {
	md, ok := changelogSince(testLog, "v0.3.2", 4)
	if !ok || !strings.HasPrefix(md, "## v0.3.2") || !strings.Contains(md, "## v0.2.0") || strings.Contains(md, "v0.3.3") {
		t.Fatalf("since v0.3.2: %v\n%s", ok, md)
	}
	md, _ = changelogSince(testLog, "v0.3.3", 4)
	if strings.Contains(md, "v0.2.0") || !strings.Contains(md, "## v0.2.1") {
		t.Fatalf("four entries: %s", md)
	}
	if _, ok := changelogSince(testLog, "v9.9.9", 4); ok {
		t.Fatal("unknown version found")
	}
}

func TestWhatsNewAfterUpdate(t *testing.T) {
	a, sv := bootVersion(t, store.State{LastRef: "bg/2/13", SeenVersion: "v0.3.2"}, "v0.3.3", testLog)
	v := plain(a)
	for _, w := range []string{"What's new in v0.3.3", "v0.3.3 — 2026-11-03", "• Copy what you tick. y opens tick boxes.", "v0.3.1", "esc close"} {
		if !strings.Contains(v, w) {
			t.Fatalf("missing %q:\n%s", w, v)
		}
	}
	// Keys go to the dialog, not the app.
	drive(a, key("s"), key("/"), key("y"))
	if !a.whatsNew.visible || a.picker.visible || a.search.visible || a.copyMenu.visible {
		t.Fatal("a key reached the app")
	}
	drive(a, key("esc"))
	if a.whatsNew.visible || strings.Contains(plain(a), "What's new in") {
		t.Fatal("esc did not close")
	}
	sv.mu.Lock()
	last := sv.calls[len(sv.calls)-1]
	sv.mu.Unlock()
	if last.SeenVersion != "v0.3.3" {
		t.Fatalf("seen version not saved: %+v", last)
	}
}

// Upgrading from a version that did not record seen_version still shows it.
func TestWhatsNewFromOlderStateWithoutSeenVersion(t *testing.T) {
	a, _ := bootVersion(t, store.State{LastRef: "bg/2/13"}, "v0.3.3", testLog)
	if !a.whatsNew.visible {
		t.Fatal("not shown")
	}
}

func TestWhatsNewNotShown(t *testing.T) {
	for name, c := range map[string]struct {
		st      store.State
		version string
	}{
		"first install": {store.State{}, "v0.3.3"},
		"same version":  {store.State{LastRef: "bg/2/13", SeenVersion: "v0.3.3"}, "v0.3.3"},
		"dev build":     {store.State{LastRef: "bg/2/13", SeenVersion: "v0.3.2"}, "dev"},
		"no entry":      {store.State{LastRef: "bg/2/13", SeenVersion: "v0.3.2"}, "v9.9.9"},
	} {
		a, _ := bootVersion(t, c.st, c.version, testLog)
		if a.whatsNew.visible {
			t.Errorf("%s: shown", name)
		}
		if name == "first install" && a.State().SeenVersion != "v0.3.3" {
			t.Errorf("first install did not record the version")
		}
	}
}

func TestWhatsNewScrolls(t *testing.T) {
	long := "## v0.3.3 — x\n\n" + strings.Repeat("- **Line.** something that happened\n", 60)
	a, _ := bootVersion(t, store.State{LastRef: "bg/2/13", SeenVersion: "v0.3.2"}, "v0.3.3", long)
	if !strings.Contains(plain(a), "j/k scroll") {
		t.Fatal("no scroll hint")
	}
	drive(a, key("j"), key("j"))
	if a.whatsNew.top != 2 {
		t.Fatalf("top %d", a.whatsNew.top)
	}
	drive(a, key("G"))
	_, vis, lines := a.whatsNewGeom()
	if a.whatsNew.top != len(lines)-vis || !strings.Contains(plain(a), "of 61") {
		t.Fatalf("G: top %d of %d (vis %d)", a.whatsNew.top, len(lines), vis)
	}
	drive(a, key("k"), key("g"))
	if a.whatsNew.top != 0 {
		t.Fatalf("g: %d", a.whatsNew.top)
	}
	drive(a, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 60, Y: 20})
	if a.whatsNew.top == 0 {
		t.Fatal("wheel did not scroll")
	}
	drive(a, tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0})
	if a.whatsNew.visible {
		t.Fatal("outside click did not close")
	}
}

// The changelog built into the program parses and has every release up to here.
func TestBuiltInChangelog(t *testing.T) {
	for _, v := range []string{"v0.3.3", "v0.3.2", "v0.3.1"} {
		if md, ok := changelogSince(vedabasetui.Changelog, v, 1); !ok || !strings.Contains(md, "- **") {
			t.Errorf("%s: %v %q", v, ok, md)
		}
	}
}

// W opens the dialog at any time; a dev build shows the newest entries.
func TestWOpensWhatsNew(t *testing.T) {
	a, _ := bootVersion(t, store.State{LastRef: "bg/2/13", SeenVersion: "v0.3.2"}, "v0.3.2", testLog)
	if a.whatsNew.visible {
		t.Fatal("shown without an update")
	}
	drive(a, key("W"))
	if v := plain(a); !a.whatsNew.visible || !strings.Contains(v, "What's new in v0.3.2") || strings.Contains(v, "v0.3.3 —") {
		t.Fatalf("W on v0.3.2:\n%s", v)
	}
	drive(a, key("esc"))
	d, _ := bootVersion(t, store.State{LastRef: "bg/2/13"}, "dev", testLog)
	drive(d, key("W"))
	if v := plain(d); !strings.Contains(v, "What's new in v0.3.3") || !strings.Contains(v, "v0.2.1") {
		t.Fatalf("W on dev:\n%s", v)
	}
	n, _ := bootVersion(t, store.State{LastRef: "bg/2/13"}, "dev", "")
	drive(n, key("W"))
	if n.whatsNew.visible {
		t.Fatal("opened with no changelog")
	}
}
