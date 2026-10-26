package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

// bootFx is boot with animations set; ticks records what the app scheduled.
func bootFx(t *testing.T, be *fakeBackend, anim bool) (*App, *[]tea.Msg) {
	t.Helper()
	var ticks []tea.Msg
	a := New(Options{Backend: be, State: store.State{LastRef: "bg/2/13", Parts: api.AllParts()},
		Settings: store.Settings{Animations: anim}})
	a.tick = func(_ time.Duration, m tea.Msg) tea.Cmd { ticks = append(ticks, m); return nil }
	drive(a, tea.WindowSizeMsg{Width: 120, Height: 40}, a.Init()())
	ticks = nil
	return a, &ticks
}

func countFrames(ms []tea.Msg) (n int) {
	for _, m := range ms {
		if _, ok := m.(frameMsg); ok {
			n++
		}
	}
	return
}

func TestNoTicksWhenIdle(t *testing.T) {
	e := Effects{enabled: true, tick: func(time.Duration, tea.Msg) tea.Cmd {
		t.Fatal("tick scheduled while idle")
		return nil
	}}
	if cmd := e.Update(frameMsg{}); cmd != nil {
		t.Fatal("idle Effects returned a Cmd")
	}
	// A live effect ticks; once it ends the ticker stops by itself.
	n := 0
	e.tick = func(time.Duration, tea.Msg) tea.Cmd { n++; return nil }
	e.Start(Spinner, time.Now())
	for i := 0; i < 5; i++ {
		e.Update(frameMsg{e.id})
	}
	if n != 6 { // the start tick plus one per frame while it lives
		t.Fatalf("ticks=%d", n)
	}
	e.Stop(Spinner)
	e.Update(frameMsg{e.id})
	if n != 6 || e.running {
		t.Fatal("ticker still running")
	}
}

func TestKeyFinishesEffects(t *testing.T) {
	a, _ := bootFx(t, newFake(t), true)
	drive(a, ToastMsg{Text: "Hello there"})
	if strings.Contains(plain(a), "Hello there") { // frame 0: still off the edge
		t.Fatal("toast should start off-screen")
	}
	drive(a, key("x")) // any key; x scrolls nothing
	if !strings.Contains(plain(a), "Hello there") {
		t.Fatal("key press did not finish the slide")
	}
}

func TestItemShowsAtFullColourImmediately(t *testing.T) {
	a, _ := bootFx(t, newFake(t), true)
	drive(a, key("l"))
	first := a.View().Content
	if strings.Contains(plain(a), "Loading") || a.reader.Empty() {
		t.Fatal("item not shown")
	}
	for i := 0; i < 5; i++ {
		drive(a, frameMsg{a.fx.id})
	}
	if a.View().Content != first {
		t.Fatal("the item changed after the first frame")
	}
}

func TestAnimationsOff(t *testing.T) {
	be := newFake(t)
	be.block["bg/1/19"] = make(chan struct{})
	a, ticks := bootFx(t, be, false)
	_ = a.Open("bg/1/19")
	drive(a, spinnerDelayMsg{"bg/1/19"})
	if countFrames(*ticks) != 0 || a.fx.Live(Spinner) || strings.Contains(plain(a), "⠋") {
		t.Fatalf("motion with animations off: %v", *ticks)
	}
	for _, m := range *ticks {
		if _, ok := m.(spinnerDelayMsg); ok {
			t.Fatal("sparkle delay scheduled")
		}
	}
	drive(a, ToastMsg{Text: "★ Added to x"})
	if a.toasts.cur == nil || a.fx.Live(Toast) || countFrames(*ticks) != 0 {
		t.Fatal("toast motion with animations off")
	}
	if !strings.Contains(plain(a), "★ Added to x") {
		t.Fatal("toast not shown")
	}
}

func TestAddShowsToastWithoutBurst(t *testing.T) {
	a, _, _ := colApp(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.fx.enabled = true
	drive(a, key("s"), key("enter"))
	typeKeys(a, "On the soul")
	drive(a, key("enter"))
	for i := 0; i < 5; i++ {
		if strings.ContainsAny(plain(a), "✦✧") {
			t.Fatal("sparkle in view")
		}
		drive(a, frameMsg{a.fx.id})
	}
	if a.toasts.cur == nil || !strings.Contains(a.toasts.cur.text, "Added to") {
		t.Fatalf("no add toast: %+v", a.toasts.cur)
	}
	if !strings.Contains(plain(a), "★ Added to") {
		t.Fatalf("toast not visible:\n%s", plain(a))
	}
}

func TestSpinnerReplacesSparkles(t *testing.T) {
	be := newFake(t)
	be.block["bg/1/19"] = make(chan struct{})
	a, _ := bootFx(t, be, true)
	_ = a.Open("bg/1/19")
	drive(a, spinnerDelayMsg{"bg/1/19"})
	has := func() string {
		v := plain(a)
		if strings.ContainsAny(v, "✦✧") {
			t.Fatal("sparkle glyph in loader")
		}
		for _, f := range spinnerFrames {
			if strings.Contains(v, f+" Loading") {
				return f
			}
		}
		t.Fatalf("no spinner frame:\n%s", v)
		return ""
	}
	f1 := has()
	drive(a, frameMsg{a.fx.id})
	if f2 := has(); f2 == f1 {
		t.Fatalf("spinner did not advance from %s", f1)
	}

	be2 := newFake(t)
	be2.block["bg/1/19"] = make(chan struct{})
	b, _ := bootFx(t, be2, false)
	_ = b.Open("bg/1/19")
	if !strings.Contains(plain(b), "⠿ Loading") {
		t.Fatalf("static spinner missing:\n%s", plain(b))
	}
}

func TestSpinnerOnlyAfterDelay(t *testing.T) {
	be := newFake(t)
	be.cached["bg/2/1"] = true
	a, ticks := bootFx(t, be, true)
	_ = a.Open("bg/2/1") // cached: no delay timer, nothing to show
	drive(a, spinnerDelayMsg{"bg/2/1"})
	if strings.ContainsAny(plain(a), "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
		t.Fatal("cached item flashed the loader")
	}

	be.block["bg/1/19"] = make(chan struct{})
	*ticks = nil
	_ = a.Open("bg/1/19")
	if strings.ContainsAny(plain(a), "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") || strings.Contains(a.View().Content, "✧") {
		t.Fatal("loader shown before the delay")
	}
	found := false
	for _, m := range *ticks {
		_, ok := m.(spinnerDelayMsg)
		found = found || ok
	}
	if !found {
		t.Fatal("no delay timer scheduled")
	}
	drive(a, spinnerDelayMsg{"bg/1/19"})
	v := plain(a)
	if !strings.ContainsAny(v, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") || !strings.Contains(plain(a), "░░░") {
		t.Fatalf("loader missing:\n%s", plain(a))
	}
	// Arrival ends it.
	drive(a, itemLoadedMsg{ref: "bg/1/19", lang: "en", item: be.item("bg/1/19")})
	if a.fx.Live(Spinner) {
		t.Fatal("sparkle outlived the load")
	}
}

func TestCountdownRetries(t *testing.T) {
	be := newFake(t)
	be.itemErr["bg/2/13"] = &api.RateLimitedError{RetryAfter: 2 * time.Second}
	a, _ := bootFx(t, be, true)
	if !strings.Contains(plain(a), "Slow down a moment") {
		t.Fatalf("view:\n%s", plain(a))
	}
	n := len(be.itemCall)
	be.mu.Lock()
	delete(be.itemErr, "bg/2/13")
	be.mu.Unlock()
	for i := 0; i < 40 && len(be.itemCall) == n; i++ {
		drive(a, frameMsg{a.fx.id})
	}
	if len(be.itemCall) <= n {
		t.Fatal("no retry after the wait")
	}
	if !a.hasItem {
		t.Fatalf("text not shown:\n%s", plain(a))
	}
	if a.cd != nil || a.fx.Live(Countdown) {
		t.Fatal("countdown outlived retry")
	}
}

func TestCountdownStaticWithoutAnimations(t *testing.T) {
	be := newFake(t)
	be.itemErr["bg/2/13"] = &api.RateLimitedError{RetryAfter: 2 * time.Second}
	a, ticks := bootFx(t, be, false)
	if v := plain(a); !strings.Contains(v, "Slow down a moment") || !strings.Contains(v, "Retrying in 2s") {
		t.Fatalf("view:\n%s", v)
	}
	if countFrames(*ticks) != 0 {
		t.Fatal("frame ticks with animations off")
	}
	seq := a.cd.seq
	drive(a, cdTickMsg{seq})
	if !strings.Contains(plain(a), "Retrying in 1s") {
		t.Fatalf("view:\n%s", plain(a))
	}
	n := len(be.itemCall)
	drive(a, cdTickMsg{seq})
	if len(be.itemCall) <= n {
		t.Fatal("no retry at zero")
	}
}

func TestSearchCountdownRetries(t *testing.T) {
	a, be := searchApp(t)
	a.fx = Effects{enabled: true, tick: func(time.Duration, tea.Msg) tea.Cmd { return nil }}
	be.searchErr = &api.RateLimitedError{RetryAfter: time.Second}
	drive(a, key("enter"))
	if v := plain(a); !strings.Contains(v, "Wait 1s before searching again") || !strings.Contains(v, "Slow down a moment") {
		t.Fatalf("view:\n%s", v)
	}
	be.mu.Lock()
	be.searchErr = nil
	n := len(be.searchCalls)
	be.mu.Unlock()
	for i := 0; i < 20 && len(be.searches()) == n; i++ {
		drive(a, frameMsg{a.fx.id})
	}
	if got := be.searches(); len(got) <= n || got[len(got)-1].q != got[0].q {
		t.Fatalf("calls=%v", got)
	}
}

func TestStartupProgressBar(t *testing.T) {
	a := New(Options{Backend: newFake(t)})
	drive(a, tea.WindowSizeMsg{Width: 80, Height: 24})
	if v := plain(a); !strings.Contains(v, "ॐ vedabase") || !strings.Contains(v, "░") {
		t.Fatalf("unknown-total view:\n%s", v)
	}
	drive(a, progressMsg{done: 2_100_000, total: 3_700_000})
	v := plain(a)
	if !strings.Contains(v, "2.1 / 3.7 MB") || !strings.Contains(v, "ॐ vedabase") || !strings.Contains(v, "█") {
		t.Fatalf("view:\n%s", v)
	}
	drive(a, progressMsg{done: 2_100_000})
	if v := plain(a); !strings.Contains(v, "2.1 MB") || strings.Contains(v, " / ") {
		t.Fatalf("indeterminate view:\n%s", v)
	}
}

func TestStartupSchedulesFrames(t *testing.T) {
	for _, anim := range []bool{true, false} {
		var ticks []tea.Msg
		a := New(Options{Backend: newFake(t), Settings: store.Settings{Animations: anim}})
		a.tick = func(_ time.Duration, m tea.Msg) tea.Cmd { ticks = append(ticks, m); return nil }
		a.Init()
		frames, loads := countFrames(ticks), 0
		for _, m := range ticks {
			if _, ok := m.(loadTickMsg); ok {
				loads++
			}
		}
		if loads == 0 || (frames > 0) != anim {
			t.Fatalf("anim=%v frames=%d loadTicks=%d", anim, frames, loads)
		}
		if anim && !a.fx.Live(Loader) {
			t.Fatal("loader not live")
		}
		drive(a, libraryMsg{lib: a.be.(*fakeBackend).lib})
		if a.fx.Live(Loader) {
			t.Fatal("loader survived the library")
		}
	}
}
