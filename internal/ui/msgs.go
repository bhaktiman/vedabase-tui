package ui

import (
	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
)

type itemLoadedMsg struct {
	ref  string
	item api.Item
	err  error
}

type renderedMsg struct {
	ref   string
	width int
	parts api.Parts
	out   string
}

type libraryMsg struct {
	lib   *library.Library
	stale bool
	err   error
}

type progressMsg struct{ done, total int64 }

// ToastMsg shows a toast; exported so overlays can emit it.
type ToastMsg struct {
	Text  string
	Error bool
}

// Internal timers and background results.
type (
	toastExpiredMsg struct{ seq int }
	resizeTickMsg   struct{ seq int }
	loadTickMsg     struct{}
	prefetchMsg     struct {
		ref string
		err error
	}
)
