package ui

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	frameDur     = 60 * time.Millisecond
	sparkleDelay = 120 * time.Millisecond // cached items never flash the loader
	burstDur     = 500 * time.Millisecond
	fadeInDur    = 150 * time.Millisecond
	toastSlide   = 3 // frames
	toastFadeDur = 300 * time.Millisecond
	burstZoneW   = 8
	mb           = 1_000_000
)

// EffectKind names an animation.
type EffectKind int

const (
	Sparkle   EffectKind = iota // loader sparkles and shimmer; lives until Stop
	Burst                       // star burst after adding to a collection
	FadeIn                      // colour ramp on a new item
	Toast                       // slide in, fade out
	Countdown                   // rate-limit wait; lives until Stop
	Loader                      // startup redraws; lives until the library arrives
)

// frameMsg is one tick of the shared 60 ms ticker. id names the ticker run it
// belongs to, so a tick left over from an earlier run is ignored.
type frameMsg struct{ id int }

// sparkleDelayMsg arrives sparkleDelay after an uncached load began.
type sparkleDelayMsg struct{ ref string }

// cdTickMsg is the once-a-second countdown tick used when motion is off.
type cdTickMsg struct{ seq int }

type effect struct {
	start  int // frame the effect began on
	frames int // lifetime in frames; 0 = until Stop
	settle bool
}

// Effects tracks the live animations and runs one shared frame ticker, which
// is scheduled only while at least one effect is live.
type Effects struct {
	enabled bool
	tick    func(d time.Duration, msg tea.Msg) tea.Cmd
	id      int
	running bool
	frame   int
	live    map[EffectKind]*effect
	began   map[EffectKind]time.Time
}

func framesFor(d time.Duration) int { return int(math.Ceil(float64(d) / float64(frameDur))) }

// Start makes an effect live and returns the Cmd that starts the ticker if it
// is not already running. With motion off nothing goes live: callers render
// the final frame.
func (e *Effects) Start(kind EffectKind, now time.Time) tea.Cmd {
	if !e.enabled {
		return nil
	}
	if e.live == nil {
		e.live, e.began = map[EffectKind]*effect{}, map[EffectKind]time.Time{}
	}
	f := 0
	switch kind {
	case Burst:
		f = framesFor(burstDur)
	case FadeIn:
		f = framesFor(fadeInDur)
	case Toast:
		f = framesFor(toastTTL)
	}
	e.live[kind] = &effect{start: e.frame, frames: f}
	e.began[kind] = now
	if e.running {
		return nil
	}
	e.running = true
	e.id++
	return e.tick(frameDur, frameMsg{e.id})
}

// Stop ends an effect.
func (e *Effects) Stop(kind EffectKind) { delete(e.live, kind) }

// Live reports whether an effect is running.
func (e *Effects) Live(kind EffectKind) bool { return e.live[kind] != nil }

// FinishAll jumps the short decorations to their final frame. Sparkle and
// Countdown track a state (loading, waiting) rather than a duration, so they
// go on; a toast keeps its place in time but skips its slide.
func (e *Effects) FinishAll() {
	delete(e.live, Burst)
	delete(e.live, FadeIn)
	if t := e.live[Toast]; t != nil {
		t.settle = true
	}
}

// Update advances the frame counter and schedules the next tick while
// anything is live.
func (e *Effects) Update(m frameMsg) tea.Cmd {
	if m.id != e.id || !e.running {
		return nil
	}
	e.frame++
	for k, f := range e.live {
		if f.frames > 0 && e.frame-f.start >= f.frames {
			delete(e.live, k)
		}
	}
	if len(e.live) == 0 {
		e.running = false
		return nil
	}
	return e.tick(frameDur, frameMsg{e.id})
}

// age is how many frames an effect has been live.
func (e *Effects) age(kind EffectKind) int {
	if f := e.live[kind]; f != nil {
		return e.frame - f.start
	}
	return 0
}

// progress is how far a timed effect has got, 0..1.
func (e *Effects) progress(kind EffectKind) float64 {
	f := e.live[kind]
	if f == nil || f.frames == 0 {
		return 1
	}
	return math.Min(float64(e.frame-f.start)/float64(f.frames), 1)
}

// mix blends a toward b by k (0..1).
func mix(a, b color.Color, k float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	l := func(x, y uint32) int { return int((float64(x>>8)*(1-k) + float64(y>>8)*k) + 0.5) }
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", l(ar, br), l(ag, bg), l(ab, bb)))
}

func (a *App) sparkleColors() []color.Color {
	return []color.Color{a.pal.Gold, a.pal.Rose, a.pal.Teal, a.pal.Violet}
}

// sparkleGlyphs is the twinkling pair shown next to the loading heading.
func (a *App) sparkleGlyphs() string {
	cs, f := a.sparkleColors(), a.fx.frame
	g := func(i int, r string) string {
		return lipgloss.NewStyle().Foreground(cs[(f/2+i)%len(cs)]).Bold(true).Render(r)
	}
	if f%2 == 0 {
		return g(0, "✦") + " " + g(2, "✧")
	}
	return g(0, "✧") + " " + g(2, "✦")
}

// shimmerLines is three placeholder lines with a brighter band moving across.
func (a *App) shimmerLines(w int) []string {
	w = max(min(w-2, 56), 8)
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	bright := lipgloss.NewStyle().Foreground(a.pal.Gold)
	band := (a.fx.frame * 2) % (w + 8)
	out := make([]string, 3)
	for i := range out {
		lw := w - i*w/6 // ragged right edge, like text
		c := band - i*3
		lo, hi := min(max(c-4, 0), lw), min(max(c, 0), lw)
		out[i] = dim.Render(strings.Repeat("░", lo)) + bright.Render(strings.Repeat("░", hi-lo)) +
			dim.Render(strings.Repeat("░", lw-hi))
	}
	return out
}

var sgr = regexp.MustCompile("\x1b\\[[0-9;:]*m")

// fade dims a rendered line: level 1 is flat Dim, level 2 is the real colours
// faint, anything else is untouched. Only visible lines come through here.
func (a *App) fade(line string, level int) string {
	switch level {
	case 1:
		return lipgloss.NewStyle().Foreground(a.pal.Dim).Render(sgr.ReplaceAllString(line, ""))
	case 2:
		const faint = "\x1b[2m"
		return faint + sgr.ReplaceAllStringFunc(line, func(s string) string {
			if s == "\x1b[m" || s == "\x1b[0m" {
				return s + faint
			}
			return s
		}) + "\x1b[m"
	}
	return line
}

// fadeLevel is the reader's dimming for this frame.
func (a *App) fadeLevel() int {
	if !a.fx.Live(FadeIn) {
		return 0
	}
	if a.fx.age(FadeIn) < 1 {
		return 1
	}
	return 2
}

// burstRow is the sparkle strip drawn left of the toast during a burst.
func (a *App) burstZone() string {
	if !a.fx.Live(Burst) {
		return ""
	}
	p := a.fx.progress(Burst)
	cells := make([]string, burstZoneW)
	for i := range cells {
		cells[i] = " "
	}
	cs := a.sparkleColors()
	for i, speed := range []float64{0.45, 0.65, 0.85, 1} {
		col := burstZoneW - 1 - int(p*speed*float64(burstZoneW-1))
		glyph := "✦"
		if (a.fx.frame+i)%2 == 1 {
			glyph = "✧"
		}
		cells[max(col, 0)] = lipgloss.NewStyle().Foreground(cs[i%len(cs)]).Render(glyph)
	}
	return strings.Join(cells, "")
}

// starPulse reports whether the tree's ★ for ref is pulsing bright this frame.
func (a *App) starPulse(ref string) bool {
	return a.fx.Live(Burst) && a.burstRef == ref && a.fx.age(Burst)%2 == 0
}

// startupBar is the first-launch gradient progress bar.
func (a *App) startupBar(done, total int64) string {
	title := lipgloss.NewStyle().Foreground(a.pal.Gold).Bold(true).Render("ॐ vedabase")
	if total <= 0 {
		dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
		return strings.Join([]string{title, "", a.sweepBar(), dim.Render(fmt.Sprintf("%.1f MB", float64(done)/mb))}, "\n")
	}
	w := max(min(a.w-4, 40), 8)
	bar := progress.New(
		progress.WithColors(a.pal.Rose, a.pal.Gold, a.pal.Teal, a.pal.Violet),
		progress.WithWidth(w), progress.WithoutPercentage(),
		progress.WithFillCharacters('█', '░'),
	)
	bar.EmptyColor = a.pal.Dim
	label := lipgloss.NewStyle().Foreground(a.pal.Dim).
		Render(fmt.Sprintf("%.1f / %.1f MB", float64(done)/mb, float64(total)/mb))
	return strings.Join([]string{title, "", bar.ViewAs(float64(done) / float64(total)), label}, "\n")
}

// cdKind says what a countdown will retry.
type cdKind int

const (
	cdReader cdKind = iota
	cdSearch
)

type countdown struct {
	kind  cdKind
	ref   string // reader: the item to reload
	gen   int    // search: the query generation to re-run
	total time.Duration
	left  time.Duration
	seq   int
}

// startCountdown shows the friendly wait and arranges the retry.
func (a *App) startCountdown(c countdown) tea.Cmd {
	c.total = max(c.total, time.Second)
	c.left = c.total
	a.cdSeq++
	c.seq = a.cdSeq
	a.cd = &c
	if a.fx.enabled {
		return a.fx.Start(Countdown, a.now())
	}
	return a.tick(time.Second, cdTickMsg{c.seq})
}

// cdAdvance moves the countdown on by d and retries at zero.
func (a *App) cdAdvance(d time.Duration) tea.Cmd {
	c := a.cd
	if c == nil {
		return nil
	}
	if c.left -= d; c.left > 0 {
		return nil
	}
	a.cd = nil
	a.fx.Stop(Countdown)
	switch c.kind {
	case cdReader:
		if c.ref == a.ref && !a.hasItem {
			return a.Open(c.ref)
		}
	case cdSearch:
		return a.search.Retry(c.gen)
	}
	return nil
}

// countdownLines is the "Slow down a moment" block: the message and a Rose bar
// shrinking over the wait, or a plain seconds line with motion off.
func (a *App) countdownLines(w int) []string {
	c := a.cd
	if c == nil {
		return nil
	}
	rose := lipgloss.NewStyle().Foreground(a.pal.Rose)
	head := rose.Bold(true).Render("Slow down a moment ☕")
	secs := int(math.Ceil(c.left.Seconds()))
	if !a.fx.enabled {
		return []string{head, lipgloss.NewStyle().Foreground(a.pal.Dim).Render(fmt.Sprintf("Retrying in %ds", secs))}
	}
	bar := progress.New(progress.WithColors(a.pal.Rose), progress.WithWidth(max(min(w-2, 30), 4)),
		progress.WithoutPercentage(), progress.WithFillCharacters('█', '░'))
	bar.EmptyColor = a.pal.Dim
	return []string{head, bar.ViewAs(float64(c.left)/float64(c.total)) + rose.Render(fmt.Sprintf(" %ds", secs))}
}

// onFrame runs the shared ticker and what rides on it.
func (a *App) onFrame(m frameMsg) tea.Cmd {
	cmd := a.fx.Update(m)
	if m.id != a.fx.id {
		return cmd
	}
	if a.cd != nil && a.fx.enabled {
		return tea.Batch(cmd, a.cdAdvance(frameDur))
	}
	return cmd
}

// sweepBar is the indeterminate bar: a gradient band sweeping across, or a
// static dim bar with motion off.
func (a *App) sweepBar() string {
	w := max(min(a.w-4, 40), 8)
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	if !a.fx.enabled {
		return dim.Render(strings.Repeat("░", w))
	}
	cs := []color.Color{a.pal.Rose, a.pal.Gold, a.pal.Teal, a.pal.Violet}
	const band = 10
	pos := a.fx.frame % (w + band)
	var b strings.Builder
	for i := 0; i < w; i++ {
		if d := pos - i; d >= 0 && d < band {
			b.WriteString(lipgloss.NewStyle().Foreground(cs[d*len(cs)/band]).Render("█"))
		} else {
			b.WriteString(dim.Render("░"))
		}
	}
	return b.String()
}
