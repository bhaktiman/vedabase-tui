package ui

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/export"
)

// exportDoneMsg reports a finished export. collection is true for a whole
// collection (the background job), false for a single item.
type exportDoneMsg struct {
	path       string
	existed    bool
	missing    int
	err        error
	collection bool
}

type exportProgressMsg struct{ done, total int }

const exportFetchTimeout = 60 * time.Second

// metaFor is the book title and chapter for ref, from the library.
func (a *App) metaFor(r string) export.Meta {
	if a.lib == nil {
		return export.Meta{}
	}
	n := a.lib.Find(r)
	if n == nil {
		if b := a.lib.Find(strings.SplitN(r, "/", 2)[0]); b != nil {
			return export.Meta{BookTitle: b.Label()}
		}
		return export.Meta{}
	}
	var m export.Meta
	if n.Parent != nil && n.Parent.Parent != nil {
		m.Chapter = n.Parent.Label()
	}
	root := n
	for root.Parent != nil {
		root = root.Parent
	}
	m.BookTitle = root.Label()
	return m
}

// tildeHome shows p with the home directory as ~.
func tildeHome(p string) string {
	if h, err := os.UserHomeDir(); err == nil && h != "" && strings.HasPrefix(p, h+string(os.PathSeparator)) {
		return "~" + p[len(h):]
	}
	return p
}

func (a *App) now() time.Time {
	if a.nowFn != nil {
		return a.nowFn()
	}
	return time.Now()
}

// exportItem writes one item (fetching it if it is not the open one).
func (a *App) exportItem(r string) tea.Cmd {
	if a.settings.ExportDir == "" {
		return a.Toast("No export directory set", true)
	}
	be, lang, dir, parts, today, meta := a.be, a.lang, a.settings.ExportDir, a.parts, a.now(), a.metaFor(r)
	var have *api.Item
	if r == a.ref && a.hasItem {
		it := a.item
		have = &it
	}
	return func() tea.Msg {
		it := have
		if it == nil {
			ctx, cancel := context.WithTimeout(context.Background(), exportFetchTimeout)
			defer cancel()
			got, _, err := be.Item(ctx, r, lang)
			if err != nil {
				return exportDoneMsg{err: err}
			}
			it = &got
		}
		path, existed, err := export.Write(dir, export.ItemFilename(r), export.ItemMarkdown(*it, meta, parts, today))
		return exportDoneMsg{path: path, existed: existed, err: err}
	}
}

// exportCollection starts the background job for a whole collection.
func (a *App) exportCollection(name string) tea.Cmd {
	if a.cols == nil {
		return a.Toast("Collections are unavailable", true)
	}
	if a.exporting {
		return a.Toast("An export is already running", false)
	}
	if a.settings.ExportDir == "" {
		return a.Toast("No export directory set", true)
	}
	col, ok := a.cols.Get(name)
	if !ok || len(col.Items) == 0 {
		return a.Toast("Nothing to export in "+name, false)
	}
	metas := make([]export.Meta, len(col.Items))
	for i, it := range col.Items {
		metas[i] = a.metaFor(it.Ref)
	}
	a.exporting, a.exportName, a.exportDone, a.exportTotal = true, name, 0, len(col.Items)
	ch := make(chan tea.Msg, len(col.Items)+2) // the job never blocks on a slow UI
	a.exportCh = ch
	go runCollectionExport(a.be, a.lang, a.settings.ExportDir, name, col.Items, metas, a.parts, a.now(), ch)
	return listenExport(ch)
}

func runCollectionExport(be Backend, lang, dir, name string, items []collections.Item, metas []export.Meta,
	parts api.Parts, today time.Time, ch chan<- tea.Msg) {
	defer close(ch)
	entries := make([]export.Entry, len(items))
	missing := 0
	for i, ci := range items {
		entries[i] = export.Entry{Ref: ci.Ref, Meta: metas[i]}
		ctx, cancel := context.WithTimeout(context.Background(), exportFetchTimeout)
		it, _, err := be.Item(ctx, ci.Ref, lang)
		cancel()
		if err != nil {
			missing++
		} else {
			entries[i].Item = &it
		}
		ch <- exportProgressMsg{done: i + 1, total: len(items)}
	}
	path, existed, err := export.Write(dir, export.CollectionFilename(name),
		export.CollectionMarkdown(name, entries, parts, today))
	ch <- exportDoneMsg{path: path, existed: existed, missing: missing, err: err, collection: true}
}

// listenExport waits for the job's next message; the App re-arms it after
// each progress message.
func listenExport(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (a *App) onExportProgress(m exportProgressMsg) tea.Cmd {
	a.exportDone, a.exportTotal = m.done, m.total
	return listenExport(a.exportCh)
}

func (a *App) onExportDone(m exportDoneMsg) tea.Cmd {
	if m.collection {
		a.exporting, a.exportCh = false, nil
	}
	if m.err != nil {
		return a.Toast("Export failed: "+m.err.Error(), true)
	}
	verb := "Exported "
	if m.existed {
		verb = "Overwrote "
	}
	text := "✓ " + verb + tildeHome(m.path)
	if m.missing > 0 {
		text += " (" + strconv.Itoa(m.missing) + " unavailable)"
	}
	return a.Toast(text, false)
}

// exportStatus is the bottom-bar text while a collection export runs.
func (a *App) exportStatus() string {
	if !a.exporting {
		return ""
	}
	return "Exporting " + truncate(a.exportName, 20) + " " + strconv.Itoa(a.exportDone) + "/" + strconv.Itoa(a.exportTotal)
}
