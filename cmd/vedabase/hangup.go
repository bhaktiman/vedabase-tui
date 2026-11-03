//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

// quitOnHangup ends the program cleanly when the terminal window closes
// (SIGHUP), so Run returns and an open note is still saved. It must only Quit:
// saving from this goroutine would race the program's Update.
func quitOnHangup(p *tea.Program) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		<-ch
		p.Quit()
	}()
}
