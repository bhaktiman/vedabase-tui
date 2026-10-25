package ui

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	updateCheckTimeout = 10 * time.Second
	updateApplyTimeout = 3 * time.Minute
)

// versionLabel is v as the sidebar shows it: "0.1.0" becomes "v0.1.0".
func versionLabel(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}

// checkUpdate asks the updater, in the background, for a newer release.
// Errors and "none" are silent.
func (a *App) checkUpdate() tea.Cmd {
	u := a.upd
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		tag, err := u.Available(ctx)
		if err != nil || tag == "" {
			return nil
		}
		return updateAvailMsg{tag}
	}
}

func (a *App) onUpdateAvail(m updateAvailMsg) tea.Cmd {
	a.avail = m.tag
	if a.state.Zen {
		return nil // nothing extra in distraction-free mode
	}
	return a.Toast("vedabase "+m.tag+" is available — press U to update", false)
}

// startUpdate downloads and installs the known release.
func (a *App) startUpdate() tea.Cmd {
	if a.upd == nil || a.avail == "" || a.updating {
		return nil
	}
	a.updating = true
	a.updDone.Store(0)
	a.updTotal.Store(0)
	u, tag := a.upd, a.avail
	apply := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateApplyTimeout)
		defer cancel()
		err := u.Apply(ctx, tag, func(done, total int64) { // another goroutine
			a.updDone.Store(done)
			a.updTotal.Store(total)
		})
		return updateDoneMsg{err}
	}
	return tea.Batch(a.Toast("Downloading vedabase "+tag+"…", false), apply, a.tick(loadRefresh, updateTickMsg{}))
}

func (a *App) onUpdateDone(m updateDoneMsg) tea.Cmd {
	a.updating = false
	if t := a.toasts.cur; t != nil && strings.HasPrefix(t.text, "Downloading") {
		a.toasts.cur, a.toasts.queue = nil, nil // the outcome replaces it at once
		a.fx.Stop(Toast)
	}
	switch {
	case m.err == nil:
		a.restart = true
		return a.quit() // saves the state, so the restart reopens here
	case errors.Is(m.err, fs.ErrPermission):
		a.updCmd = installCmd
		return a.Toast("No permission to replace the app — run the command below", true)
	}
	return a.Toast("Update failed: "+m.err.Error()+" — nothing changed", true)
}
