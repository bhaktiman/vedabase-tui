// Package ui is the Bubble Tea front end: a contents tree and a reader.
package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/render"
	"vedabase-tui/internal/store"
)

type Backend interface {
	Item(ctx context.Context, ref string) (api.Item, bool, error)
	Cached(ref string) bool
	Library(ctx context.Context, progress func(done, total int64)) (*library.Library, bool, error)
	RefreshLibrary(ctx context.Context) (*library.Library, error)
	Search(ctx context.Context, q, book string, page int) (api.SearchPage, error)
	Page(ctx context.Context, url string) ([]byte, error)
}

type Options struct {
	Backend     Backend
	Collections *collections.Store // may be nil: no stars, no collections
	Settings    store.Settings
	State       store.State
	SaveState   func(store.State) error
	Now         func() time.Time
	Toasts      []ToastMsg // shown first, e.g. startup warnings
}

// loadRefresh redraws the startup screen with motion off.
const loadRefresh = 250 * time.Millisecond

const loadingStatus = "Loading…"

const (
	resizeQuiet = 100 * time.Millisecond
	minTreeCols = 70 // below this width the tree is hidden

	minFrameCols = 50 // below this width or minFrameRows the panes go unframed
	minFrameRows = 10
)

// App is the root model.
type App struct {
	be       Backend
	cols     *collections.Store
	settings store.Settings
	state    store.State
	save     func(store.State) error
	pal      render.Palette
	rend     *render.Renderer
	w, h     int

	lib       *library.Library
	colViews  []library.CollectionView // what the sidebar's collections root last got
	tree      Tree
	libErr    error
	progDone  atomic.Int64
	progTotal atomic.Int64
	startup   []ToastMsg

	ref        string
	node       *library.Node
	item       api.Item
	hasItem    bool
	status     string // shown instead of the document: Loading…, errors
	reader     Reader
	renderedW  int
	parts      api.Parts
	restoreRef string // first render of this ref scrolls to restore
	restore    int
	savedRef   string // the ref state.json was last written for on open

	search      Search
	picker      Picker
	colview     CollectionsView
	themes      themePicker
	gotoBar     Goto
	filtering   bool // the tree filter line is being edited
	filterIn    textinput.Model
	help        bool
	nowFn       func() time.Time
	exporting   bool
	exportName  string
	exportDone  int
	exportTotal int
	exportCh    chan tea.Msg

	focusTree  bool
	seq        keySeq
	resizeSeq  int
	prefetched map[string]bool
	scr        scraped
	toasts     toaster
	opener     func(string) error // starts the browser; nil means openURL

	fx    Effects
	cd    *countdown
	cdSeq int

	tick func(d time.Duration, msg tea.Msg) tea.Cmd // replaced in tests
}

func New(o Options) *App {
	a := &App{
		be: o.Backend, cols: o.Collections, settings: o.Settings, state: o.State, save: o.SaveState,
		pal: render.Default, rend: render.NewRenderer(render.Default),
		w: 80, h: 24, parts: o.State.Parts, startup: o.Toasts,
		prefetched: map[string]bool{}, scr: newScraped(), nowFn: o.Now,
		tick: func(d time.Duration, msg tea.Msg) tea.Cmd {
			return tea.Tick(d, func(time.Time) tea.Msg { return msg })
		},
	}
	a.fx = Effects{enabled: o.Settings.Animations, tick: func(d time.Duration, m tea.Msg) tea.Cmd { return a.tick(d, m) }}
	if a.parts == (api.Parts{}) && !o.State.PartsSet { // a state file with no parts saved: show everything
		a.parts = api.AllParts()
	}
	if p, ok := render.ThemeByName(o.State.Theme); ok {
		a.pal = p
		a.rend = render.NewRenderer(p)
	}
	a.search = NewSearch(a.pal)
	a.search.be = o.Backend
	a.picker = NewPicker(a.pal)
	a.picker.now = a.now
	a.colview = NewCollectionsView(a.pal)
	a.gotoBar = NewGoto()
	a.filterIn = lineInput("filter")
	for _, in := range []*textinput.Model{&a.search.input, &a.picker.input, &a.colview.input, &a.filterIn, &a.gotoBar.input} {
		themeInput(in, a.pal)
	}
	a.layout()
	return a
}

// State is what gets persisted on quit.
func (a *App) State() store.State {
	s := store.State{LastRef: a.ref, Scroll: a.reader.offset, Parts: a.parts, PartsSet: true, TreeHidden: a.state.TreeHidden, Wide: a.state.Wide, Theme: a.state.Theme}
	if a.restoreRef == a.ref {
		s.Scroll = a.restore // the first render has not happened yet
	}
	if a.ref == "" {
		s.LastRef = a.state.LastRef
	}
	return s
}

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.loadLibrary(), a.tick(loadRefresh, loadTickMsg{}), a.fx.Start(Loader, a.now())}
	for _, t := range a.startup {
		cmds = append(cmds, a.Toast(t.Text, t.Error))
	}
	return tea.Batch(cmds...)
}

func (a *App) loadLibrary() tea.Cmd {
	a.libErr = nil
	return func() tea.Msg {
		lib, stale, err := a.be.Library(context.Background(), func(done, total int64) {
			a.progDone.Store(done)
			a.progTotal.Store(total)
		})
		return libraryMsg{lib: lib, stale: stale, err: err}
	}
}

func (a *App) refreshLibrary() tea.Cmd {
	return func() tea.Msg {
		lib, err := a.be.RefreshLibrary(context.Background())
		return libraryMsg{lib: lib, err: err}
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		return a, a.resize(m.Width, m.Height)
	case tea.KeyPressMsg:
		cmd := a.onKey(m)
		a.syncCollections()
		return a, cmd
	case tea.MouseClickMsg:
		cmd := a.onMouse(tea.Mouse(m), true)
		a.syncCollections()
		return a, cmd
	case tea.MouseWheelMsg:
		return a, a.onMouse(tea.Mouse(m), false)
	case libraryMsg:
		return a, a.onLibrary(m)
	case loadTickMsg:
		if a.lib == nil && a.libErr == nil {
			return a, a.tick(loadRefresh, loadTickMsg{})
		}
	case itemLoadedMsg:
		return a, a.onItem(m)
	case renderedMsg:
		return a, a.onRendered(m)
	case resizeTickMsg:
		if m.seq == a.resizeSeq {
			return a, a.renderCmd()
		}
	case prefetchMsg:
		if m.err != nil {
			delete(a.prefetched, m.ref) // let a later display try again
		}
	case searchResultsMsg:
		cmd := a.search.Update(m)
		if d, gen := a.search.takeRateLimit(); d > 0 {
			cmd = tea.Batch(cmd, a.startCountdown(countdown{kind: cdSearch, gen: gen, total: d}))
		}
		return a, cmd
	case titlesMsg:
		return a, a.onTitles(m)
	case ExpandLazyMsg:
		return a, a.onExpandLazy(m.Node)
	case yearsMsg:
		return a, a.onYears(m)
	case yearPageMsg:
		return a, a.onYearPage(m)
	case searchOpenMsg:
		a.focusTree = false
		return a, a.Open(m.ref)
	case OpenMsg:
		a.focusTree = false
		return a, a.open(m.Node.Ref, m.Node)
	case exportCollectionMsg:
		return a, a.exportCollection(m.name)
	case exportProgressMsg:
		return a, a.onExportProgress(m)
	case exportDoneMsg:
		return a, a.onExportDone(m)
	case openedMsg:
		return a, a.onOpened(m)
	case ToastMsg:
		return a, a.Toast(m.Text, m.Error)
	case frameMsg:
		return a, a.onFrame(m)
	case spinnerDelayMsg:
		if m.ref == a.ref && !a.hasItem && a.status == loadingStatus {
			return a, a.fx.Start(Spinner, a.now())
		}
	case cdTickMsg:
		if c := a.cd; c != nil && m.seq == c.seq && !a.fx.enabled {
			cmd := a.cdAdvance(time.Second)
			if a.cd != nil {
				cmd = tea.Batch(cmd, a.tick(time.Second, cdTickMsg{m.seq}))
			}
			return a, cmd
		}
	case progressMsg:
		a.progDone.Store(m.done)
		a.progTotal.Store(m.total)
	case toastExpiredMsg:
		return a, a.toastExpired(m)
	}
	return a, nil
}

func (a *App) onLibrary(m libraryMsg) tea.Cmd {
	if m.err != nil || m.lib == nil {
		if a.lib == nil {
			if m.err == nil {
				m.err = errors.New("no library")
			}
			a.libErr = m.err
			a.fx.Stop(Loader)
		}
		return nil // a failed background refresh keeps the library we have
	}
	a.fx.Stop(Loader)
	var cmds []tea.Cmd
	if m.stale {
		cmds = append(cmds, a.refreshLibrary())
	}
	first := a.lib == nil
	a.lib = m.lib
	a.colViews = nil
	a.setCollections()
	if first {
		a.tree = NewTree(m.lib, a.starred, a.pal)
		a.layout()
		cmds = append(cmds, a.openStart())
	} else {
		a.tree.SetLibrary(m.lib)
		a.refindNode()   // nodes are new objects in the new library
		a.applyScraped() // the new tree has none of the scraped titles
	}
	return tea.Batch(cmds...)
}

// collectionViews is the collections store as the library wants it.
func (a *App) collectionViews() []library.CollectionView {
	if a.cols == nil {
		return nil
	}
	var out []library.CollectionView
	for _, c := range a.cols.List() {
		v := library.CollectionView{Name: c.Name}
		for _, it := range c.Items {
			v.Items = append(v.Items, library.CollectionItem{Ref: it.Ref, Book: it.Book, Label: it.Label})
		}
		out = append(out, v)
	}
	return out
}

// setCollections puts the store's collections into the library's root, unless
// that is what it already has. It reports whether it changed anything.
func (a *App) setCollections() bool {
	if a.lib == nil {
		return false
	}
	v := a.collectionViews()
	if v == nil {
		v = []library.CollectionView{}
	}
	if a.colViews != nil && reflect.DeepEqual(v, a.colViews) {
		return false
	}
	a.colViews = v
	a.lib.SetCollections(v)
	return true
}

// syncCollections refreshes the sidebar after the collections may have changed.
func (a *App) syncCollections() {
	if a.setCollections() {
		a.tree.SetLibrary(a.lib)
		a.refindNode()
		a.layout()
	}
}

// refindNode points a.node at the current library's node for a.ref: the item
// under its collection when the reader was opened from one, else the book's.
func (a *App) refindNode() {
	if a.lib == nil || a.ref == "" {
		return
	}
	if name, ok := library.InCollection(a.node); ok {
		if n := a.lib.CollectionItem(name, a.ref); n != nil {
			a.node = n
			return
		}
	}
	a.node = a.lib.Find(a.ref)
}

func (a *App) starred(ref string) bool { return a.cols != nil && a.cols.Contains(ref) }

// openStart opens the last-read item, or the first leaf of the Gītā.
func (a *App) openStart() tea.Cmd {
	if r := a.state.LastRef; r != "" && a.lib.Find(r) != nil {
		a.restoreRef, a.restore = r, a.state.Scroll
		return a.Open(r)
	}
	n := a.lib.Find("bg")
	for n != nil && !n.Leaf && len(n.Children) > 0 {
		n = n.Children[0]
	}
	if n == nil || !n.Leaf {
		return nil
	}
	return a.Open(n.Ref)
}

// Open makes ref the current item and loads it.
func (a *App) Open(ref string) tea.Cmd { return a.open(ref, nil) }

// open is Open for an item reached through from, a collection item node: the
// reader then belongs to that collection (breadcrumb, h/l order).
func (a *App) open(ref string, from *library.Node) tea.Cmd {
	a.ref, a.hasItem = ref, false
	a.reader.Clear()
	if a.restoreRef != ref {
		a.restoreRef = ""
	}
	if a.lib != nil {
		a.node = a.lib.Find(ref)
		if _, ok := library.InCollection(from); ok && from.Ref == ref {
			a.node = from
		}
		a.tree.Reveal(a.node)
		a.layout() // Reveal may have cleared the filter line
	}
	a.fx.Stop(Spinner)
	if a.cd != nil && a.cd.kind == cdReader {
		a.cd = nil
		a.fx.Stop(Countdown)
	}
	var delay tea.Cmd
	if !a.be.Cached(ref) {
		a.status = loadingStatus
		if a.fx.enabled {
			delay = a.tick(spinnerDelay, spinnerDelayMsg{ref})
		}
	} else {
		a.status = ""
	}
	be := a.be
	return tea.Batch(delay, a.ancestorTitlesCmd(a.node), func() tea.Msg {
		it, _, err := be.Item(context.Background(), ref)
		return itemLoadedMsg{ref: ref, item: it, err: err}
	})
}

func (a *App) onItem(m itemLoadedMsg) tea.Cmd {
	if m.ref != a.ref {
		return nil
	}
	a.fx.Stop(Spinner)
	if m.err != nil {
		return a.itemFailed(m.err)
	}
	a.item, a.hasItem, a.status = m.item, true, ""
	a.setItemTitle(a.node, m.item)
	return a.renderCmd()
}

func (a *App) itemFailed(err error) tea.Cmd {
	var rl *api.RateLimitedError
	switch {
	case errors.Is(err, api.ErrOffline):
		a.status = "You're offline and this hasn't been downloaded yet."
		return nil
	case errors.As(err, &rl):
		a.status = fmt.Sprintf("vedabase.cc asked us to wait %ds", int(math.Ceil(rl.RetryAfter.Seconds())))
		return a.startCountdown(countdown{kind: cdReader, ref: a.ref, total: rl.RetryAfter})
	}
	a.status = err.Error()
	return a.Toast(err.Error(), true)
}

// renderCmd renders the current item in the background; the reply is dropped
// if the ref, width or parts changed meanwhile.
func (a *App) renderCmd() tea.Cmd {
	w := a.textW()
	if !a.hasItem || w <= 0 {
		return nil
	}
	ref, parts, item, rend, pal := a.ref, a.parts, a.item, a.rend, a.pal.Name
	return func() tea.Msg {
		md := render.DisplayMarkdown(item, parts)
		out, err := rend.Render(ref, md, w, parts)
		if err != nil {
			out = md
		}
		return renderedMsg{ref: ref, width: w, parts: parts, pal: pal, out: out}
	}
}

func (a *App) onRendered(m renderedMsg) tea.Cmd {
	if m.ref != a.ref || m.width != a.textW() || m.parts != a.parts || m.pal != a.pal.Name {
		return nil
	}
	fresh := a.reader.Empty()
	a.reader.SetContent(m.out, !fresh)
	a.renderedW = m.width
	if a.restoreRef == a.ref {
		a.reader.offset = a.restore
		a.reader.clamp()
		a.restoreRef = ""
	}
	a.persistOnOpen()
	return a.prefetch()
}

// prefetch fetches the next item in reading order once, for the cache only.
func (a *App) prefetch() tea.Cmd {
	if a.lib == nil || a.node == nil {
		return nil
	}
	next := a.lib.Next(a.node)
	if next == nil || a.prefetched[next.Ref] || a.be.Cached(next.Ref) {
		return nil
	}
	a.prefetched[next.Ref] = true
	be, ref := a.be, next.Ref
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _, err := be.Item(ctx, ref)
		return prefetchMsg{ref: ref, err: err}
	}
}

// step opens the next (d=1) or previous (d=-1) item, or toasts at a book's end.
func (a *App) step(d int) tea.Cmd {
	if a.lib == nil || a.node == nil {
		return nil
	}
	n := a.lib.Next(a.node)
	edge := "End of "
	if d < 0 {
		n, edge = a.lib.Prev(a.node), "Start of "
	}
	if n == nil {
		root := a.node
		for root.Parent != nil {
			root = root.Parent
		}
		if name, ok := library.InCollection(a.node); ok {
			return a.Toast(edge+name, false)
		}
		return a.Toast(edge+root.Label(), false)
	}
	return a.open(n.Ref, n)
}

func (a *App) togglePart(k string) tea.Cmd {
	p := &a.parts
	switch k {
	case "1":
		p.Devanagari = !p.Devanagari
	case "2":
		p.Verse = !p.Verse
	case "3":
		p.Synonyms = !p.Synonyms
	case "4":
		p.Translation = !p.Translation
	case "5":
		p.Purport = !p.Purport
	}
	a.persist()
	return a.renderCmd()
}

func (a *App) quit() tea.Cmd {
	if a.save != nil {
		_ = a.save(a.State())
	}
	return tea.Quit
}

// openSearch shows the search overlay for the current book.
func (a *App) openSearch(withInput bool) tea.Cmd {
	code, title := "", ""
	if a.node != nil {
		root := a.lib.Find(a.ref) // a collection item sits under the collections root
		if root == nil {
			root = a.node
		}
		for root.Parent != nil {
			root = root.Parent
		}
		code, title = root.Book, root.Label()
	}
	return a.search.Show(code, title, withInput)
}

func (a *App) onKey(kp tea.KeyPressMsg) tea.Cmd {
	a.fx.FinishAll()
	if a.lib != nil && kp.String() != "ctrl+c" {
		switch {
		case a.help:
			a.help = false
			return nil
		case a.gotoBar.visible:
			return a.gotoKey(kp)
		case a.filtering:
			return a.filterKey(kp)
		}
	}
	if a.lib != nil && a.search.visible && kp.String() != "ctrl+c" {
		return a.search.Update(kp)
	}
	if a.lib != nil && kp.String() != "ctrl+c" {
		switch {
		case a.picker.visible:
			return a.picker.Update(kp)
		case a.colview.visible:
			return a.colview.Update(kp)
		case a.themes.visible:
			return a.themeKey(kp)
		}
	}
	k := a.seq.Resolve(kp)
	switch k {
	case "ctrl+c", "q":
		return a.quit()
	}
	if a.lib == nil {
		if k == "r" && a.libErr != nil {
			return tea.Batch(a.loadLibrary(), a.tick(loadRefresh, loadTickMsg{}), a.fx.Start(Loader, a.now()))
		}
		return nil
	}
	switch k {
	case "t":
		a.state.TreeHidden = !a.state.TreeHidden
		a.focusTree = a.focusTree && a.treeVisible()
		a.persist()
		return a.relayout()
	case "w":
		a.state.Wide = !a.state.Wide
		a.persist()
		msg := "Centred column"
		if a.state.Wide {
			msg = "Full width"
		}
		return tea.Batch(a.relayout(), a.Toast(msg, false))
	case "tab":
		a.focusTree = !a.focusTree && a.treeVisible()
		return nil
	case "1", "2", "3", "4", "5":
		return a.togglePart(k)
	case "/":
		return a.openSearch(true)
	case ":":
		return a.gotoBar.Show()
	case "?":
		a.help = true
		return nil
	case "T":
		a.openThemes()
		return nil
	case "f":
		if a.focusTree && a.treeVisible() {
			return a.startFilter()
		}
		return nil
	case "s":
		return a.openPicker()
	case "'":
		if a.cols == nil {
			return a.Toast("Collections are unavailable", true)
		}
		a.colview.Show(a.cols)
		return nil
	case "z":
		a.filtering = false
		a.filterIn.Blur()
		a.tree.CollapseAll()
		a.layout()
		return nil
	case "e":
		if n := a.tree.Selected(); a.focusTree && a.treeVisible() && n != nil && n.Parent != nil &&
			n.Parent.Ref == library.CollectionsRef {
			return a.exportCollection(n.Seg)
		}
		if r := a.target(); r != "" {
			return a.exportItem(r)
		}
		return nil
	case "o":
		return a.openItemWeb()
	case "n":
		return a.search.Next(1)
	case "N":
		return a.search.Next(-1)
	case "esc":
		if a.search.Live() {
			return a.openSearch(false)
		}
	}
	if a.focusTree && a.treeVisible() {
		return a.treeUpdate(a.tree.Selected(), func(t Tree) (Tree, tea.Cmd) { return t.Update(kp) })
	}
	switch k {
	case "h", "left":
		return a.step(-1)
	case "l", "right":
		return a.step(1)
	case "r":
		if a.ref != "" && !a.hasItem && a.status != loadingStatus {
			return a.Open(a.ref)
		}
	default:
		a.reader.Key(k)
	}
	return nil
}

// treeUpdate applies f to the tree and does what a change there calls for:
// titles for the newly opened branch sel (the node f acts on), the next page
// of letters.
func (a *App) treeUpdate(sel *library.Node, f func(Tree) (Tree, tea.Cmd)) tea.Cmd {
	was := sel != nil && a.tree.expanded[sel.Ref]
	var cmd tea.Cmd
	a.tree, cmd = f(a.tree)
	cmds := []tea.Cmd{cmd, a.nextYearPage()}
	if sel != nil && !was && a.tree.expanded[sel.Ref] {
		cmds = append(cmds, a.titlesCmd(sel))
	}
	return tea.Batch(cmds...)
}

// target is the item s and e act on: the selected tree leaf when the tree
// has focus, else the open item.
func (a *App) target() string {
	if a.focusTree && a.treeVisible() {
		if n := a.tree.Selected(); n != nil && n.Leaf {
			return n.Ref
		}
		return ""
	}
	return a.ref
}

func (a *App) openPicker() tea.Cmd {
	if a.cols == nil {
		return a.Toast("Collections are unavailable", true)
	}
	r := a.target()
	if r == "" {
		return nil
	}
	var it api.Item
	ok := false
	switch {
	case r == a.ref && a.hasItem:
		it, ok = a.item, true
	case a.be.Cached(r):
		if got, _, err := a.be.Item(context.Background(), r); err == nil {
			it, ok = got, true
		}
	}
	book := strings.SplitN(r, "/", 2)[0]
	return a.picker.Show(a.cols, collections.Item{Ref: r, Book: book, Label: itemLabel(r, it, ok)})
}
