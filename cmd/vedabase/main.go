package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/notes"
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

	// Collections and notes live in data_dir when it is set. Files left in the
	// old folder are never moved: a synced folder may already hold another
	// computer's copy, so the user moves them.
	// An unusable data_dir turns both stores off rather than falling back to
	// the default folder, where saves would be stranded.
	dataDir, dwarn, dataOK := store.DataDir(settings, paths.Data)
	if dwarn != "" {
		toasts = append(toasts, ui.ToastMsg{Text: dwarn, Error: true})
	}
	if dataOK {
		if msg := store.LeftBehind(dataDir, paths.Data); msg != "" {
			toasts = append(toasts, ui.ToastMsg{Text: msg, Error: true})
		}
	}

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

	var cols *collections.Store
	var nts *notes.Store
	if dataOK {
		var warning string
		cols, warning, err = collections.Open(filepath.Join(dataDir, "collections.json"))
		switch {
		case err != nil:
			cols = nil // run with collections disabled
			toasts = append(toasts, ui.ToastMsg{Text: "collections disabled: " + err.Error(), Error: true})
		case warning != "":
			toasts = append(toasts, ui.ToastMsg{Text: warning, Error: true})
		}

		var nwarn string
		var nerr error
		nts, nwarn, nerr = notes.Open(filepath.Join(dataDir, "notes.json"))
		switch {
		case nerr != nil:
			nts = nil // run with notes disabled
			toasts = append(toasts, ui.ToastMsg{Text: "notes disabled: " + nerr.Error(), Error: true})
		case nwarn != "":
			toasts = append(toasts, ui.ToastMsg{Text: nwarn, Error: true})
		}
	}

	svc := &service.Service{
		Client: api.NewClient(service.DefaultBase, "vedabase-tui/"+version, nil),
		Items:  store.ItemCache{Dir: filepath.Join(paths.Cache, "items-v2"), OldDir: filepath.Join(paths.Cache, "items")},
		Blobs:  store.BlobCache{Dir: paths.Cache},
	}
	app := ui.New(ui.Options{
		Backend:     svc,
		Collections: cols,
		Notes:       nts,
		Settings:    settings,
		State:       store.LoadState(statePath),
		SaveState:   func(s store.State) error { return store.SaveState(statePath, s) },
		Toasts:      toasts,
		Version:     ver,
		Updater:     updater,
		InstallDir:  installDir,
	})
	p := tea.NewProgram(app)
	quitOnHangup(p)
	_, err = p.Run()
	// A note still open when the program ended any way but a key.
	if ferr := app.FlushNote(); ferr != nil {
		keepUnsaved(app, []string{dataDir, paths.Data, os.TempDir()}, ferr)
	}
	if err != nil {
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

// keepUnsaved writes the note text that could not be saved to the first of
// dirs that takes it (the data folder can be the very place that failed), and
// says where it went. If none does, the text itself goes to stderr, so closing
// the program never loses it.
func keepUnsaved(app *ui.App, dirs []string, cause error) {
	r, text, ok := app.OpenNote()
	if !ok {
		return
	}
	name := "notes.json.unsaved-" + time.Now().Format("20060102-150405")
	var errs []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, name)
		err := os.MkdirAll(dir, 0o755)
		if err == nil {
			err = os.WriteFile(path, []byte(text), 0o600)
		}
		if err == nil {
			fmt.Fprintf(os.Stderr, "vedabase: could not save your note on %s: %v; kept in %s\n", r, cause, path)
			return
		}
		errs = append(errs, err.Error())
	}
	fmt.Fprintf(os.Stderr, "vedabase: could not save your note on %s: %v; and could not keep it in a file (%s). Your note:\n%s\n",
		r, cause, strings.Join(errs, "; "), text)
}
