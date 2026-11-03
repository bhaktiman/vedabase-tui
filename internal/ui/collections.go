package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/collections"
	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/render"
)

const (
	newCollectionRow = "+ New collection…"
	labelMax         = 60
)

// jumpCollectionMsg asks the App to show a collection (the `c` picker's
// Enter), or, when ref is set, to open that item inside it.
type jumpCollectionMsg struct{ name, ref string }

// jumpRow is one row of the `c` picker: a collection, or (ref set) one of
// its items.
type jumpRow struct{ col, ref string }

// exportCollectionMsg asks the App to export a collection to one file.
type exportCollectionMsg struct{ name string }

func toastCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return ToastMsg{Text: text, Error: isErr} }
}

// itemLabel is what a collection stores for an item so it renders offline:
// "Bg 2.13 — first line of the translation", or the title for documents.
func itemLabel(r string, it api.Item, ok bool) string {
	d := ref.Display(r)
	if !ok {
		return d
	}
	first := ""
	for _, l := range strings.Split(it.Translation, "\n") {
		if l = oneLine(strings.ReplaceAll(l, "**", "")); l != "" {
			first = l
			break
		}
	}
	if first == "" {
		return d
	}
	if rs := []rune(first); len(rs) > labelMax {
		first = string(rs[:labelMax-1]) + "…"
	}
	if it.Document != nil {
		return first
	}
	return d + " — " + first
}

func newInput(placeholder string) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	st := in.Styles()
	st.Cursor.Blink = false
	in.SetStyles(st)
	return in
}

// listView is how many rows listBox shows and the first one, for n rows and
// the cursor.
func listView(h, n, cursor int) (vis, top int) {
	vis = max(min(h-2, 24)-5, 1)
	return vis, max(min(cursor-vis+1, n-vis), 0)
}

// listBox draws a titled, scrolling list in a bordered box.
func listBox(pal render.Palette, w, h int, title string, rows []string, cursor int, extra, hint string) string {
	bw := max(min(w-4, 80), 24)
	inner := bw - 4
	dim := lipgloss.NewStyle().Foreground(pal.Dim)
	vis, top := listView(h, len(rows), cursor)
	lines := []string{gradientBold(title, stops(pal)...)}
	if extra != "" {
		lines = append(lines, extra)
	}
	clip := lipgloss.NewStyle().MaxWidth(inner)
	for i := top; i < len(rows) && i < top+vis; i++ {
		mark := "  "
		if i == cursor {
			mark = lipgloss.NewStyle().Foreground(pal.Accent).Render("▸ ")
		}
		lines = append(lines, clip.Render(mark+rows[i]))
	}
	lines = append(lines, dim.Render(truncate(hint, inner)))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(pal.Dim).
		Padding(0, 1).Width(bw).Render(strings.Join(lines, "\n"))
}

// Picker is the `s` overlay: choose a collection for one item, or make one.
type Picker struct {
	pal   render.Palette
	store *collections.Store
	now   func() time.Time
	w, h  int

	visible  bool
	jump     bool // the `c` "Go to collection" mode: no new-collection row
	alpha    bool // A–Z instead of most recently used first; kept while the app runs
	item     collections.Item
	names    []string
	rows     []jumpRow // jump mode: each collection, then its items
	cursor   int
	typing   bool
	input    textinput.Model
	feedback string
	hasNote  func(string) bool // set by the App: whether a ref has a note
}

func NewPicker(pal render.Palette) Picker {
	return Picker{pal: pal, input: newInput("name of the new collection"), now: time.Now}
}

func (p *Picker) SetSize(w, h int) { p.w, p.h = w, h }

// Show opens the picker for it (Added is filled in when chosen).
func (p *Picker) Show(st *collections.Store, it collections.Item) tea.Cmd {
	p.visible, p.store, p.item, p.jump = true, st, it, false
	p.sortNames()
	p.cursor = 0
	p.typing = false
	p.input.SetValue("")
	p.input.Blur()
	return nil
}

// ShowJump opens the picker in jump mode: choosing a collection emits
// jumpCollectionMsg instead of adding an item.
func (p *Picker) ShowJump(st *collections.Store) tea.Cmd {
	p.Show(st, collections.Item{})
	p.jump = true
	p.sortNames()
	return nil
}

// rowCount is the number of selectable rows: the names, plus the
// new-collection row unless jumping.
func (p *Picker) rowCount() int {
	if p.jump {
		return len(p.rows)
	}
	return len(p.names) + 1
}

func (p *Picker) close() { p.visible = false; p.typing = false; p.input.Blur() }

// sortNames lists the collections A–Z or, by default, the one an item was
// last added to first (an empty one counts from its creation).
func (p *Picker) sortNames() {
	cols := p.store.List()
	used := func(c collections.Collection) time.Time {
		if len(c.Items) == 0 {
			return c.Created
		}
		var t time.Time
		for _, it := range c.Items {
			if it.Added.After(t) {
				t = it.Added
			}
		}
		return t
	}
	slices.SortStableFunc(cols, func(a, b collections.Collection) int {
		if !p.alpha {
			if c := used(b).Compare(used(a)); c != 0 {
				return c
			}
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	p.names, p.rows = p.names[:0], p.rows[:0]
	for _, c := range cols {
		p.names = append(p.names, c.Name)
		if p.jump {
			p.rows = append(p.rows, jumpRow{col: c.Name})
			for _, it := range c.Items {
				p.rows = append(p.rows, jumpRow{col: c.Name, ref: it.Ref})
			}
		}
	}
}

func (p *Picker) add(name string) tea.Cmd {
	it := p.item
	it.Added = p.now()
	added, err := p.store.Add(name, it)
	p.close()
	switch {
	case err != nil:
		return toastCmd("Couldn't save: "+err.Error(), true)
	case !added:
		return toastCmd("Already in "+name, false)
	}
	return func() tea.Msg { return ToastMsg{Text: "★ Added to " + name} }
}

func (p *Picker) Update(kp tea.KeyPressMsg) tea.Cmd {
	k := kp.String()
	if p.typing {
		switch k {
		case "esc":
			p.typing = false
			p.input.Blur()
		case "enter":
			if name := strings.TrimSpace(p.input.Value()); name != "" {
				return p.add(name)
			}
		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(kp)
			return cmd
		}
		return nil
	}
	switch k {
	case "esc", "q":
		p.close()
	case "j", "down":
		p.cursor = min(p.cursor+1, max(p.rowCount()-1, 0))
	case "k", "up":
		p.cursor = max(p.cursor-1, 0)
	case "tab": // the cursor stays on the same collection (or item)
		if p.jump {
			var cur jumpRow
			if p.cursor < len(p.rows) {
				cur = p.rows[p.cursor]
			}
			p.alpha = !p.alpha
			p.sortNames()
			if i := slices.Index(p.rows, cur); i >= 0 {
				p.cursor = i
			}
			return nil
		}
		cur := ""
		if p.cursor < len(p.names) {
			cur = p.names[p.cursor]
		}
		p.alpha = !p.alpha
		p.sortNames()
		if i := slices.Index(p.names, cur); i >= 0 {
			p.cursor = i
		}
	case "enter":
		if p.jump {
			if p.cursor < len(p.rows) {
				r := p.rows[p.cursor]
				p.close()
				return func() tea.Msg { return jumpCollectionMsg{r.col, r.ref} }
			}
			return nil
		}
		if p.cursor < len(p.names) {
			return p.add(p.names[p.cursor])
		}
		p.typing = true
		return p.input.Focus()
	}
	return nil
}

// Click handles a left click at x, y inside the box: a row is selected, and
// the selected row again is chosen.
func (p *Picker) Click(x, y int) tea.Cmd {
	if p.typing {
		return nil
	}
	n := p.rowCount()
	vis, top := listView(p.h, n, p.cursor)
	i := top + y - 2
	if y < 2 || y-2 >= vis || i >= n {
		return nil
	}
	if i == p.cursor {
		return p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	p.cursor = i
	return nil
}

// Wheel moves the cursor by d rows.
func (p *Picker) Wheel(d int) {
	if !p.typing {
		p.cursor = min(max(p.cursor+d, 0), max(p.rowCount()-1, 0))
	}
}

func (p Picker) View() string {
	rows := append(append([]string(nil), p.names...), newCollectionRow)
	sort := "recent first (tab: A–Z)" // says the order in use, then what Tab gives
	if p.alpha {
		sort = "A–Z (tab: recent first)"
	}
	extra, hint := "", "enter choose · j/k move · "+sort+" · esc close"
	if p.jump {
		dim := lipgloss.NewStyle().Foreground(p.pal.Dim)
		fg := lipgloss.NewStyle().Foreground(p.pal.Fg)
		bold := fg.Bold(true)
		rows = rows[:0]
		for _, r := range p.rows {
			col, _ := p.store.Get(r.col)
			if r.ref == "" {
				rows = append(rows, bold.Render(r.col)+dim.Render(fmt.Sprintf("  ·  %d", len(col.Items))))
				continue
			}
			for _, it := range col.Items {
				if it.Ref == r.ref {
					rows = append(rows, "  "+fg.Render(it.Label)+dim.Render(noteMark(p.hasNote, it.Ref)+"  ·  "+it.Book))
					break
				}
			}
		}
		hint = "enter open · j/k move · " + sort + " · esc close"
		return listBox(p.pal, p.w, p.h, "Go to collection", rows, p.cursor, "", hint)
	}
	if p.typing {
		p.input.SetWidth(max(min(p.w-4, 80)-8, 1))
		rows = p.names
		extra = lipgloss.NewStyle().Foreground(p.pal.Dim).Render("New: ") + p.input.View()
		hint = "enter add · esc back"
		return listBox(p.pal, p.w, p.h, "Add to collection", rows, -1, extra, hint)
	}
	return listBox(p.pal, p.w, p.h, "Add to collection", rows, p.cursor, extra, hint)
}

// CollectionsView is the `'` overlay: browse, open, remove, rename, delete,
// export.
type CollectionsView struct {
	pal   render.Palette
	store *collections.Store
	w, h  int

	visible bool
	in      string // collection being browsed; "" at the top level
	cursor  int
	saved   int    // list cursor to return to
	confirm string // "d" or "D" while waiting for y
	rename  bool
	input   textinput.Model
	hasNote func(string) bool // set by the App: whether a ref has a note
}

func NewCollectionsView(pal render.Palette) CollectionsView {
	return CollectionsView{pal: pal, input: newInput("new name")}
}

func (c *CollectionsView) SetSize(w, h int) { c.w, c.h = w, h }

func (c *CollectionsView) Show(st *collections.Store) {
	c.visible, c.store, c.in, c.cursor, c.confirm, c.rename = true, st, "", 0, "", false
}

// ShowIn opens the overlay inside the named collection; Esc returns to its
// place in the list.
func (c *CollectionsView) ShowIn(st *collections.Store, name string) {
	c.Show(st)
	for i, col := range st.List() {
		if col.Name == name {
			c.saved = i
		}
	}
	c.in = name
}

// atCollection opens the top level with the cursor on name; false if it is gone.
func (c *CollectionsView) atCollection(st *collections.Store, name string) bool {
	c.Show(st)
	for i, col := range st.List() {
		if col.Name == name {
			c.cursor = i
			return true
		}
	}
	return false
}

// StartRename opens the overlay renaming the named collection.
func (c *CollectionsView) StartRename(st *collections.Store, name string) tea.Cmd {
	if !c.atCollection(st, name) {
		return nil
	}
	c.rename = true
	c.input.SetValue(name)
	return c.input.Focus()
}

// StartDelete opens the overlay asking to delete the named collection.
func (c *CollectionsView) StartDelete(st *collections.Store, name string) {
	if c.atCollection(st, name) {
		c.confirm = "D"
	}
}

// itemName is how a confirm prompt names an item: its label up to the " — "
// that starts the first line of text, or the reference when it has no label.
func itemName(it collections.Item) string {
	name, _, _ := strings.Cut(it.Label, " — ")
	if name = strings.TrimSpace(name); name == "" {
		return ref.Display(it.Ref)
	}
	return truncate(name, labelMax)
}

func (c *CollectionsView) close() { c.visible = false; c.input.Blur() }

func (c *CollectionsView) count() int {
	if c.in == "" {
		return len(c.store.List())
	}
	col, _ := c.store.Get(c.in)
	return len(col.Items)
}

func (c *CollectionsView) clamp() { c.cursor = max(min(c.cursor, c.count()-1), 0) }

func (c *CollectionsView) selectedName() (string, bool) {
	l := c.store.List()
	if c.in != "" || c.cursor >= len(l) {
		return "", false
	}
	return l[c.cursor].Name, true
}

func (c *CollectionsView) selectedItem() (collections.Item, bool) {
	col, _ := c.store.Get(c.in)
	if c.in == "" || c.cursor >= len(col.Items) {
		return collections.Item{}, false
	}
	return col.Items[c.cursor], true
}

func (c *CollectionsView) Update(kp tea.KeyPressMsg) tea.Cmd {
	k := kp.String()
	switch {
	case c.rename:
		return c.onRename(kp)
	case c.confirm != "":
		return c.onConfirm(k)
	}
	switch k {
	case "esc", "q":
		if c.in != "" {
			c.in, c.cursor = "", c.saved
			c.clamp()
		} else {
			c.close()
		}
	case "j", "down":
		c.cursor++
		c.clamp()
	case "k", "up":
		c.cursor--
		c.clamp()
	case "enter":
		if name, ok := c.selectedName(); ok {
			c.saved, c.in, c.cursor = c.cursor, name, 0
		} else if it, ok := c.selectedItem(); ok {
			c.close()
			return func() tea.Msg { return searchOpenMsg{it.Ref} }
		}
	case "J", "K":
		if it, ok := c.selectedItem(); ok {
			d := 1
			if k == "K" {
				d = -1
			}
			if err := c.store.Move(c.in, it.Ref, d); err != nil {
				return toastCmd("Couldn't save: "+err.Error(), true)
			}
			col, _ := c.store.Get(c.in)
			for i, x := range col.Items {
				if x.Ref == it.Ref {
					c.cursor = i
				}
			}
			c.clamp()
		}
	case "d":
		if _, ok := c.selectedItem(); ok {
			c.confirm = "d"
		}
	case "D":
		if _, ok := c.selectedName(); ok {
			c.confirm = "D"
		}
	case "R":
		if name, ok := c.selectedName(); ok {
			c.rename = true
			c.input.SetValue(name)
			return c.input.Focus()
		}
	case "e":
		if name, ok := c.selectedName(); ok {
			return func() tea.Msg { return exportCollectionMsg{name} }
		}
		if c.in != "" {
			name := c.in
			return func() tea.Msg { return exportCollectionMsg{name} }
		}
	}
	return nil
}

func (c *CollectionsView) onConfirm(k string) tea.Cmd {
	what := c.confirm
	c.confirm = ""
	if k != "y" {
		return nil
	}
	var err error
	if what == "d" {
		if it, ok := c.selectedItem(); ok {
			err = c.store.Remove(c.in, it.Ref)
		}
	} else if name, ok := c.selectedName(); ok {
		err = c.store.Delete(name)
	}
	c.clamp()
	if err != nil {
		return toastCmd("Couldn't save: "+err.Error(), true)
	}
	return nil
}

func (c *CollectionsView) onRename(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.String() {
	case "esc":
		c.rename = false
		c.input.Blur()
	case "enter":
		old, ok := c.selectedName()
		name := strings.TrimSpace(c.input.Value())
		if !ok || name == "" {
			return nil
		}
		c.rename = false
		c.input.Blur()
		if name == old {
			return nil
		}
		if err := c.store.Rename(old, name); err != nil {
			return toastCmd(err.Error(), true)
		}
		c.clamp()
	default:
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(kp)
		return cmd
	}
	return nil
}

// Click handles a left click at x, y inside the box: a row is selected, and
// the selected row again is activated, as Enter does.
func (c *CollectionsView) Click(x, y int) tea.Cmd {
	if c.rename || c.confirm != "" {
		return nil
	}
	vis, top := listView(c.h, max(c.count(), 1), c.cursor)
	i := top + y - 2
	if y < 2 || y-2 >= vis || i >= c.count() {
		return nil
	}
	if i == c.cursor {
		return c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	c.cursor = i
	return nil
}

// Wheel moves the cursor by d rows.
func (c *CollectionsView) Wheel(d int) {
	if !c.rename && c.confirm == "" {
		c.cursor += d
		c.clamp()
	}
}

func (c CollectionsView) View() string {
	dim := lipgloss.NewStyle().Foreground(c.pal.Dim)
	fg := lipgloss.NewStyle().Foreground(c.pal.Fg)
	var rows []string
	title, extra := "Collections", ""
	hint := "enter open · e export · R rename · D delete · esc close"
	if c.in == "" {
		for _, col := range c.store.List() {
			rows = append(rows, fg.Render(col.Name)+dim.Render(fmt.Sprintf("  ·  %d", len(col.Items))))
		}
		if len(rows) == 0 {
			rows = []string{dim.Render("No collections yet — press s while reading")}
		}
	} else {
		title = c.in
		hint = "enter open · J/K move · d remove · e export · esc back"
		col, _ := c.store.Get(c.in)
		for _, it := range col.Items {
			rows = append(rows, fg.Render(it.Label)+dim.Render(noteMark(c.hasNote, it.Ref)+"  ·  "+it.Book))
		}
		if len(rows) == 0 {
			rows = []string{dim.Render("Empty")}
		}
	}
	switch {
	case c.rename:
		c.input.SetWidth(max(min(c.w-4, 80)-14, 1))
		extra = dim.Render("Rename to: ") + c.input.View()
		hint = "enter rename · esc cancel"
	case c.confirm == "d":
		it, _ := c.selectedItem()
		extra = lipgloss.NewStyle().Foreground(c.pal.Rose).Render("Remove " + itemName(it) + "? y/n")
	case c.confirm == "D":
		name, _ := c.selectedName()
		extra = lipgloss.NewStyle().Foreground(c.pal.Rose).Render("Delete " + name + "? y/n")
	}
	return listBox(c.pal, c.w, c.h, title, rows, c.cursor, extra, hint)
}

// noteMark is " ✎" after an item that has a note, else "".
func noteMark(has func(string) bool, ref string) string {
	if has != nil && has(ref) {
		return " ✎"
	}
	return ""
}
