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
	e.Start(FadeIn, time.Now())
	for i := 0; i < 5; i++ {
		e.Update(frameMsg{e.id})
	}
	if n != 3 { // the start tick plus two before the 3-frame fade ended
		t.Fatalf("ticks=%d", n)
	}
	if e.running || e.Live(FadeIn) {
		t.Fatal("ticker still running")
	}
}

func TestKeyFinishesEffects(t *testing.T) {
	a, _ := bootFx(t, newFake(t), true)
	for i := 0; i < 5; i++ { // let the boot fade run out
		drive(a, frameMsg{a.fx.id})
	}
	final := a.View().Content
	a.fx.Start(FadeIn, time.Now())
	if a.fadeLevel() == 0 || a.View().Content == final {
		t.Fatal("fade not visible")
	}
	drive(a, key("x")) // any key; x scrolls nothing, so the view can be compared
	if a.fx.Live(FadeIn) || a.View().Content != final {
		t.Fatalf("effect survived a key press")
	}
}

func TestFadeOnNewItemOnly(t *testing.T) {
	a, _ := bootFx(t, newFake(t), true)
	drive(a, key("l"))
	if !a.fx.Live(FadeIn) {
		t.Fatal("new item did not fade in")
	}
}

func TestAnimationsOff(t *testing.T) {
	be := newFake(t)
	be.block["bg/1/19"] = make(chan struct{})
	a, ticks := bootFx(t, be, false)
	_ = a.Open("bg/1/19")
	drive(a, sparkleDelayMsg{"bg/1/19"})
	if countFrames(*ticks) != 0 || a.fx.Live(Sparkle) || strings.Contains(plain(a), "✦") {
		t.Fatalf("motion with animations off: %v", *ticks)
	}
	for _, m := range *ticks {
		if _, ok := m.(sparkleDelayMsg); ok {
			t.Fatal("sparkle delay scheduled")
		}
	}
	// Star add: toast shown, no burst.
	drive(a, ToastMsg{Text: "★ Added to x", Star: "bg/2/13"})
	if a.toasts.cur == nil || a.fx.Live(Burst) || a.fx.Live(Toast) || countFrames(*ticks) != 0 {
		t.Fatal("burst or toast motion with animations off")
	}
	if !strings.Contains(plain(a), "★ Added to x") {
		t.Fatal("toast not shown")
	}
}

func TestBurstAndToastMotion(t *testing.T) {
	a, _ := bootFx(t, newFake(t), true)
	drive(a, ToastMsg{Text: "★ Added to x", Star: "bg/2/13"})
	if !a.fx.Live(Burst) || !a.fx.Live(Toast) {
		t.Fatal("effects not started")
	}
	if strings.Contains(plain(a), "Added to x") { // frame 0: still off the edge
		t.Fatal("toast should start off-screen")
	}
	for i := 0; i < 3; i++ {
		drive(a, frameMsg{a.fx.id})
	}
	v := plain(a)
	if !strings.Contains(v, "★ Added to x") || !strings.ContainsAny(v, "✦✧") {
		t.Fatalf("settled toast with sparkles missing:\n%s", v)
	}
	for i := 0; i < 20; i++ {
		drive(a, frameMsg{a.fx.id})
	}
	if a.fx.Live(Burst) {
		t.Fatal("burst should end after ~500ms")
	}
}

func TestSparkleOnlyAfterDelay(t *testing.T) {
	be := newFake(t)
	be.cached["bg/2/1"] = true
	a, ticks := bootFx(t, be, true)
	_ = a.Open("bg/2/1") // cached: no delay timer, nothing to show
	drive(a, sparkleDelayMsg{"bg/2/1"})
	if strings.Contains(a.View().Content, "✦") {
		t.Fatal("cached item flashed the loader")
	}

	be.block["bg/1/19"] = make(chan struct{})
	*ticks = nil
	_ = a.Open("bg/1/19")
	if strings.Contains(a.View().Content, "✦") || strings.Contains(a.View().Content, "✧") {
		t.Fatal("loader shown before the delay")
	}
	found := false
	for _, m := range *ticks {
		_, ok := m.(sparkleDelayMsg)
		found = found || ok
	}
	if !found {
		t.Fatal("no delay timer scheduled")
	}
	drive(a, sparkleDelayMsg{"bg/1/19"})
	v := a.View().Content
	if !strings.ContainsAny(v, "✦✧") || !strings.Contains(plain(a), "░░░") {
		t.Fatalf("loader missing:\n%s", plain(a))
	}
	// Arrival ends it.
	drive(a, itemLoadedMsg{ref: "bg/1/19", item: be.item("bg/1/19")})
	if a.fx.Live(Sparkle) {
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
	drive(a, tea.WindowSizeMsg{Width: 80, Height: 24}, progressMsg{done: 2_100_000, total: 3_700_000})
	v := plain(a)
	if !strings.Contains(v, "2.1 / 3.7 MB") || !strings.Contains(v, "ॐ vedabase") || !strings.Contains(v, "█") {
		t.Fatalf("view:\n%s", v)
	}
}
