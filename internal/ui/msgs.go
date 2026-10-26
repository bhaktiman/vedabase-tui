package ui

import (
	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
)

type itemLoadedMsg struct {
	ref  string
	lang string
	item api.Item
	err  error
}

type renderedMsg struct {
	ref   string
	width int
	parts api.Parts
	pal   string // palette name the render started with
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
	toastExpiredMsg  struct{ seq int }
	resizeTickMsg    struct{ seq int }
	loadTickMsg      struct{}
	updateTickMsg    struct{} // redraws the bottom bar while the download runs
	updateCheckedMsg struct { // a check's answer; tag "" for none
		tag string
		err error
	}
	updateDoneMsg struct{ err error } // the end of Apply
	prefetchMsg   struct {
		ref string
		err error
	}
)
