package ui

import (
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
	return a.tick(toastTTL, toastExpiredMsg{a.toasts.seq})
}

func (a *App) toastExpired(m toastExpiredMsg) tea.Cmd {
	if m.seq != a.toasts.seq {
		return nil
	}
	a.toasts.cur = nil
	if len(a.toasts.queue) == 0 {
		return nil
	}
	next := a.toasts.queue[0]
	a.toasts.queue = a.toasts.queue[1:]
	return a.show(next)
}

// toastView renders the visible toast within max cells, or "" when none.
func (a *App) toastView(max int) string {
	t := a.toasts.cur
	if t == nil || max < 4 {
		return ""
	}
	bg := a.pal.Teal
	if t.err {
		bg = a.pal.Rose
	}
	return lipgloss.NewStyle().Background(bg).Foreground(a.pal.Bg).Bold(true).
		Render(" " + truncate(t.text, max-2) + " ")
}
