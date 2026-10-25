package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	toastTTL   = 2500 * time.Millisecond
	toastQueue = 4
)

type toast struct {
	text string
	err  bool
}

// toaster shows one toast at a time; later ones wait their turn.
type toaster struct {
	cur   *toast
	queue []toast
	seq   int // identifies the visible toast, so a stale expiry is ignored
}

// Toast queues a toast; the returned Cmd expires the visible one.
func (a *App) Toast(text string, isErr bool) tea.Cmd {
	t := toast{text, isErr}
	if a.toasts.cur == nil {
		return a.show(t)
	}
	last := a.toasts.cur.text
	if n := len(a.toasts.queue); n > 0 {
		last = a.toasts.queue[n-1].text
	}
	if last != text && len(a.toasts.queue) < toastQueue {
		a.toasts.queue = append(a.toasts.queue, t)
	}
	return nil
}

func (a *App) show(t toast) tea.Cmd {
	a.toasts.cur = &t
	a.toasts.seq++
	return tea.Batch(a.fx.Start(Toast, a.now()), a.tick(toastTTL, toastExpiredMsg{a.toasts.seq}))
}

func (a *App) toastExpired(m toastExpiredMsg) tea.Cmd {
	if m.seq != a.toasts.seq {
		return nil
	}
	a.toasts.cur = nil
	a.fx.Stop(Toast)
	if len(a.toasts.queue) == 0 {
		return nil
	}
	next := a.toasts.queue[0]
	a.toasts.queue = a.toasts.queue[1:]
	return a.show(next)
}

// toastView renders the visible toast within max cells, or "" when none. In
// its last 300 ms it fades into the background, and a live burst pulses the ★.
func (a *App) toastView(max int) string {
	t := a.toasts.cur
	if t == nil || max < 4 {
		return ""
	}
	bg := a.pal.Teal
	if t.err {
		bg = a.pal.Rose
	}
	fg := a.pal.Bg
	if a.fx.Live(Toast) {
		left := toastTTL - time.Duration(a.fx.age(Toast))*frameDur
		if left < toastFadeDur {
			k := 1 - float64(max0(left))/float64(toastFadeDur)
			bg, fg = mix(bg, a.pal.Bg, k), mix(fg, a.pal.Bg, k)
		}
	}
	st := lipgloss.NewStyle().Background(bg).Foreground(fg).Bold(true)
	text := truncate(t.text, max-2)
	if rest, ok := strings.CutPrefix(text, "★"); ok && a.fx.Live(Burst) {
		star := st
		if a.fx.age(Burst)%2 == 0 {
			star = st.Foreground(a.pal.Bright)
		}
		return st.Render(" ") + star.Render("★") + st.Render(rest+" ")
	}
	return st.Render(" " + text + " ")
}

func max0(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

// placeToast puts the toast at the right end of base (a line w cells wide),
// sliding it in from the edge and trailing burst sparkles.
func (a *App) placeToast(base string, w int) string {
	t := a.toastView(w)
	if t == "" {
		return base
	}
	zone := ""
	if w >= 40 {
		zone = a.burstZone()
	}
	tw := lipgloss.Width(t)
	zw := 0
	if zone != "" {
		zw = burstZoneW
	}
	shift := 0
	if f := a.slideFrames(); f < toastSlide {
		shift = (toastSlide - f) * (tw + zw) / toastSlide
	}
	keep := max(w-tw-zw-1, 0) + shift
	line := padTo(lipgloss.NewStyle().MaxWidth(max(keep, 1)).Render(base), keep) + " " + zone + t
	return lipgloss.NewStyle().MaxWidth(max(w, 1)).Render(line)
}

// slideFrames is how many slide frames the visible toast has had.
func (a *App) slideFrames() int {
	ef := a.fx.live[Toast]
	if ef == nil || ef.settle {
		return toastSlide
	}
	return a.fx.age(Toast)
}
