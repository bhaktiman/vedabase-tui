package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

// linkApp boots on bg/2/13, whose translation cites Bg. 4.7 and whose purport
// is long enough to scroll.
func linkApp(t *testing.T, w, h int, st store.State, width int) *App {
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
	it.Translation = "See Bg. 4.7 for the answer."
	it.Purport = strings.Repeat("A long paragraph of purport text that goes on and on.\n\n", 40)
	it.References = []api.Reference{{Text: "Bg. 4.7", Ref: "bg/4/7", Field: "translation"}}
	be.items["bg/2/13"] = it
	if st.Parts == (api.Parts{}) {
		st.Parts = api.AllParts()
	}
	st.LastRef = "bg/2/13"
	a := New(Options{Backend: be, State: st, Settings: store.Settings{Mouse: true, ReadingWidth: width}})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: w, Height: h}, a.Init()())
	return a
}

// cellOf finds s on the screen as plain text and returns its first cell.
func cellOf(t *testing.T, a *App, s string) (x, y int) {
	t.Helper()
	for y, l := range strings.Split(plain(a), "\n") {
		if i := strings.Index(l, s); i >= 0 {
			return lipgloss.Width(l[:i]), y
		}
	}
	t.Fatalf("%q not on screen:\n%s", s, plain(a))
	return
}

func TestClickLinkOpensInEveryLayout(t *testing.T) {
	cases := []struct {
		name  string
		w, h  int
		st    store.State
		width int
	}{
		{"sidebar centred", 120, 40, store.State{}, 60},
		{"sidebar full width", 120, 40, store.State{Wide: true}, 60},
		{"no sidebar centred", 120, 40, store.State{TreeHidden: true}, 60},
		{"no sidebar full width", 120, 40, store.State{TreeHidden: true, Wide: true}, 60},
		{"zen", 120, 40, store.State{Zen: true}, 60},
		{"zen full width", 120, 40, store.State{Zen: true, Wide: true}, 60},
		{"unframed small", 45, 12, store.State{Parts: api.Parts{Translation: true}}, 30},
		{"wide sidebar", 120, 40, store.State{TreeWidth: 38}, 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := linkApp(t, c.w, c.h, c.st, c.width)
			if c.st.Zen && !a.state.Zen {
				t.Fatal("not zen")
			}
			x, y := cellOf(t, a, "Bg. 4.7")
			for _, dx := range []int{0, 3, 6} { // first, middle and last cell
				a := linkApp(t, c.w, c.h, c.st, c.width)
				drive(a, click(x+dx, y))
				if a.ref != "bg/4/7" {
					t.Fatalf("click at +%d: ref = %q", dx, a.ref)
				}
			}
			// Just outside the link on either side does nothing.
			a = linkApp(t, c.w, c.h, c.st, c.width)
			drive(a, click(x+len("Bg. 4.7"), y), click(x-1, y))
			if a.ref != "bg/2/13" {
				t.Fatalf("click beside the link opened %q", a.ref)
			}
		})
	}
}

func TestClickLinkAfterScroll(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	_, y0 := cellOf(t, a, "Bg. 4.7")
	drive(a, key("j"))
	x, y := cellOf(t, a, "Bg. 4.7")
	if y != y0-1 {
		t.Fatalf("link moved %d -> %d", y0, y)
	}
	drive(a, click(x, y))
	if a.ref != "bg/4/7" {
		t.Fatalf("ref = %q", a.ref)
	}
}

func TestClickLinkNeedsMouseSetting(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	a.settings.Mouse = false
	x, y := cellOf(t, a, "Bg. 4.7")
	drive(a, click(x, y))
	if a.ref != "bg/2/13" {
		t.Fatalf("opened %q with the mouse off", a.ref)
	}
}

func TestBackRestoresRefAndOffset(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	drive(a, key("j"), key("j"), key("j"))
	if a.reader.Offset() != 3 {
		t.Fatalf("offset = %d", a.reader.Offset())
	}
	drive(a, a.follow("bg/4/7"))
	if a.ref != "bg/4/7" || len(a.back) != 1 {
		t.Fatalf("ref=%q back=%d", a.ref, len(a.back))
	}
	drive(a, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if a.ref != "bg/2/13" || a.reader.Offset() != 3 {
		t.Fatalf("after back ref=%q offset=%d", a.ref, a.reader.Offset())
	}
	if len(a.back) != 0 || len(a.fwd) != 1 {
		t.Fatalf("back=%d fwd=%d", len(a.back), len(a.fwd))
	}
}

func TestBracketKeysGoBackAndForward(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	drive(a, key("j"), key("j"))
	drive(a, a.follow("bg/4/7"))
	drive(a, key("["))
	if a.ref != "bg/2/13" || a.reader.Offset() != 2 {
		t.Fatalf("[ : ref=%q offset=%d", a.ref, a.reader.Offset())
	}
	drive(a, key("j"), key("]"))
	if a.ref != "bg/4/7" {
		t.Fatalf("] : ref=%q", a.ref)
	}
	if len(a.back) != 1 || len(a.fwd) != 0 {
		t.Fatalf("back=%d fwd=%d", len(a.back), len(a.fwd))
	}
	drive(a, key("["))
	if a.ref != "bg/2/13" || a.reader.Offset() != 3 {
		t.Fatalf("[ again: ref=%q offset=%d", a.ref, a.reader.Offset())
	}
}

func TestBackAndForwardWithEmptyStacksDoNothing(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	for _, k := range []tea.Msg{key("["), key("]"), tea.KeyPressMsg{Code: tea.KeyBackspace}} {
		drive(a, k)
	}
	if a.ref != "bg/2/13" || a.toasts.cur != nil {
		t.Fatalf("ref=%q toast=%v", a.ref, a.toasts.cur)
	}
}

func TestBackWorksFromTreeFocus(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	drive(a, a.follow("bg/4/7"))
	a.focusTree = true
	drive(a, key("["))
	if a.ref != "bg/2/13" {
		t.Fatalf("ref = %q", a.ref)
	}
}

func TestForwardClearedByOtherOpens(t *testing.T) {
	opens := map[string]func(a *App){
		"l":    func(a *App) { drive(a, key("l")) },
		"goto": func(a *App) { drive(a, a.Open("bg/2/20")) },
	}
	for name, open := range opens {
		a := linkApp(t, 120, 40, store.State{}, 60)
		drive(a, a.follow("bg/4/7"))
		drive(a, key("["))
		if len(a.fwd) != 1 {
			t.Fatalf("%s: fwd = %d before", name, len(a.fwd))
		}
		open(a)
		if len(a.fwd) != 0 {
			t.Fatalf("%s: forward kept", name)
		}
		drive(a, key("]"))
		if a.ref == "bg/4/7" {
			t.Fatalf("%s: ] went forward", name)
		}
	}
}

func TestFollowClearsForwardAndKeepsBack(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	drive(a, a.follow("bg/4/7"), a.follow("bg/5/1"))
	drive(a, key("["))
	if len(a.back) != 1 || len(a.fwd) != 1 {
		t.Fatalf("back=%d fwd=%d", len(a.back), len(a.fwd))
	}
	drive(a, a.follow("bg/6/1"))
	if len(a.fwd) != 0 || len(a.back) != 2 {
		t.Fatalf("back=%d fwd=%d", len(a.back), len(a.fwd))
	}
}

func TestOtherOpenKeepsBackStack(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	drive(a, a.follow("bg/4/7"), key("l"))
	if len(a.back) != 1 {
		t.Fatalf("back = %d", len(a.back))
	}
	drive(a, key("["))
	if a.ref != "bg/2/13" {
		t.Fatalf("ref = %q", a.ref)
	}
}

func TestHistoryCappedAt100(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	for i := 0; i < 130; i++ {
		a.follow("bg/4/7")
	}
	if len(a.back) != 100 {
		t.Fatalf("back = %d", len(a.back))
	}
}

func TestBackHintOnlyWithHistory(t *testing.T) {
	a := linkApp(t, 200, 40, store.State{}, 60)
	if strings.Contains(ansiRe.ReplaceAllString(bottomLine(a), ""), "⌫ back") {
		t.Fatal("back hint without history")
	}
	drive(a, a.follow("bg/4/7"))
	txt := ansiRe.ReplaceAllString(bottomLine(a), "")
	if !strings.Contains(txt, "⌫ back") {
		t.Fatalf("no back hint: %q", txt)
	}
	if strings.Index(txt, "⌫ back") > strings.Index(txt, "/ search") {
		t.Fatalf("back hint should lead: %q", txt)
	}
	drive(a, key("["))
	if strings.Contains(ansiRe.ReplaceAllString(bottomLine(a), ""), "⌫ back") {
		t.Fatal("back hint with empty stack")
	}
	// Short of room it goes like the other hints.
	drive(a, a.follow("bg/4/7"), tea.WindowSizeMsg{Width: 60, Height: 40})
	if w := lipgloss.Width(bottomLine(a)); w != 60 {
		t.Fatalf("bar width %d", w)
	}
}

func TestHelpMentionsHistory(t *testing.T) {
	a := linkApp(t, 120, 40, store.State{}, 60)
	drive(a, key("?"))
	v := plain(a)
	for _, w := range []string{"Backspace / [", "Back after following a link", "Forward"} {
		if !strings.Contains(v, w) {
			t.Errorf("help lacks %q", w)
		}
	}
	if n := len(strings.Split(v, "\n")); n > 40 {
		t.Errorf("help is %d rows", n)
	}
	// One column, on a narrow screen, still fits the height it is given.
	if h := lipgloss.Height(a.helpView()); h > 50 {
		t.Errorf("single-column help is %d rows", h)
	}
}
