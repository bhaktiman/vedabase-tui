package ui

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/update"
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

// checkUpdate asks the updater, in the background, for a newer release. The
// startup check is silent about errors and "none"; checkNow reports them.
func (a *App) checkUpdate() tea.Cmd {
	a.checking = true
	u := a.upd
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		tag, err := u.Available(ctx)
		return updateCheckedMsg{tag, err}
	}
}

// checkNow is U before any newer release is known: check, and say what came of it.
func (a *App) checkNow() tea.Cmd {
	switch {
	case a.upd == nil:
		return a.Toast("Updates are unavailable: the app could not find its own file", true)
	case !update.Release(a.version):
		return a.Toast("This is a development build ("+a.version+"); it does not update", false)
	case a.updating, a.checking && a.report: // already asked
		return nil
	}
	toast := a.Toast("Checking for updates…", false)
	a.report = true
	if a.checking { // the startup check is still out; report its answer
		return toast
	}
	return tea.Batch(toast, a.checkUpdate())
}

func (a *App) onUpdateChecked(m updateCheckedMsg) tea.Cmd {
	report := a.report
	a.checking, a.report = false, false
	if report {
		if t := a.toasts.cur; t != nil && strings.HasPrefix(t.text, "Checking") {
			a.toasts.cur, a.toasts.queue = nil, nil // the answer replaces it at once
			a.fx.Stop(Toast)
		}
	}
	switch {
	case m.err == nil && m.tag != "":
		return a.onUpdateAvail(m.tag)
	case !report:
		return nil
	case m.err != nil:
		return a.Toast("Couldn't check for updates: "+m.err.Error(), true)
	}
	return a.Toast("vedabase "+a.version+" is up to date", false)
}

// updateHint is what to show when the binary in dir cannot be replaced: the
// install command aimed at that directory, or the releases page on Windows,
// which install.sh does not support.
func updateHint(goos, dir, home string) string {
	if goos == "windows" {
		return releasesURL
	}
	if dir == "" || (home != "" && dir == filepath.Join(home, ".local", "bin")) {
		return installCmd
	}
	return "curl -fsSL " + releasesURL + "/download/install.sh | sudo INSTALL_DIR=" + dir + " sh"
}

func (a *App) onUpdateAvail(tag string) tea.Cmd {
	a.avail = tag
	return a.Toast("vedabase "+tag+" is available — press U to update", false)
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
		a.updatedTo = a.avail
		return a.quit() // saves the state, so the restart reopens here
	case errors.Is(m.err, fs.ErrPermission):
		home, _ := os.UserHomeDir()
		a.updCmd = updateHint(runtime.GOOS, a.installDir, home)
		return a.Toast("No permission to replace the app — run the command below", true)
	case errors.Is(m.err, update.ErrPartial):
		return a.Toast("Update failed: "+m.err.Error(), true)
	}
	return a.Toast("Update failed: "+m.err.Error()+" — nothing changed", true)
}
