package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/service"
	"vedabase-tui/internal/store"
	"vedabase-tui/internal/ui"
	"vedabase-tui/internal/update"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("vedabase " + version)
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "vedabase:", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := store.DefaultPaths()
	if err != nil {
		return err
	}
	var toasts []ui.ToastMsg

	settings, err := store.LoadSettings(filepath.Join(paths.Config, "config.toml"))
	if err != nil {
		toasts = append(toasts, ui.ToastMsg{Text: "config.toml: " + err.Error(), Error: true})
	}
	statePath := filepath.Join(paths.State, "state.json")

	// A failed lookup only turns updates off; the reader still works.
	ver := version // as the sidebar shows it, and as the updater compares it
	if ver != "" && ver[0] >= '0' && ver[0] <= '9' {
		ver = "v" + ver
	}
	exe, exeErr := update.Executable()
	installDir := ""
	var updater ui.Updater
	if exeErr == nil {
		installDir = filepath.Dir(exe)
		update.Cleanup(exe)
		updater = &update.Updater{Repo: "bhaktiman/vedabase-tui", Current: ver, Exe: exe}
	}

	cols, warning, err := collections.Open(filepath.Join(paths.Data, "collections.json"))
	switch {
	case err != nil:
		cols = nil // run with collections disabled
		toasts = append(toasts, ui.ToastMsg{Text: "collections disabled: " + err.Error(), Error: true})
	case warning != "":
		toasts = append(toasts, ui.ToastMsg{Text: warning, Error: true})
	}

	svc := &service.Service{
		Client: api.NewClient(service.DefaultBase, "vedabase-tui/"+version, nil),
		Items:  store.ItemCache{Dir: filepath.Join(paths.Cache, "items")},
		Blobs:  store.BlobCache{Dir: paths.Cache},
	}
	app := ui.New(ui.Options{
		Backend:     svc,
		Collections: cols,
		Settings:    settings,
		State:       store.LoadState(statePath),
		SaveState:   func(s store.State) error { return store.SaveState(statePath, s) },
		Toasts:      toasts,
		Version:     ver,
		Updater:     updater,
		InstallDir:  installDir,
	})
	if _, err := tea.NewProgram(app).Run(); err != nil {
		return err
	}
	if app.RestartRequested() {
		if err := update.Restart(exe); err != nil {
			fmt.Fprintf(os.Stderr, "vedabase: updated to %s — start vedabase again\n", app.UpdatedTo())
		}
		return nil
	}
	return nil
}
