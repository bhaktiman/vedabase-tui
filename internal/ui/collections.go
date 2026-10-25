package ui

import (
	"fmt"
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

// listBox draws a titled, scrolling list in a bordered box.
func listBox(pal render.Palette, w, h int, title string, rows []string, cursor int, extra, hint string) string {
	bw := max(min(w-4, 80), 24)
	inner := bw - 4
	dim := lipgloss.NewStyle().Foreground(pal.Dim)
	vis := max(min(h-2, 24)-5, 1)
	top := max(min(cursor-vis+1, len(rows)-vis), 0)
	lines := []string{lipgloss.NewStyle().Foreground(pal.Gold).Bold(true).Render(title)}
	if extra != "" {
		lines = append(lines, extra)
	}
	clip := lipgloss.NewStyle().MaxWidth(inner)
	for i := top; i < len(rows) && i < top+vis; i++ {
		mark := "  "
		if i == cursor {
			mark = lipgloss.NewStyle().Foreground(pal.Gold).Render("▸ ")
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
	item     collections.Item
	names    []string
	cursor   int
	typing   bool
	input    textinput.Model
	feedback string
}

func NewPicker(pal render.Palette) Picker {
	return Picker{pal: pal, input: newInput("name of the new collection"), now: time.Now}
}

func (p *Picker) SetSize(w, h int) { p.w, p.h = w, h }

// Show opens the picker for it (Added is filled in when chosen).
func (p *Picker) Show(st *collections.Store, it collections.Item) tea.Cmd {
	p.visible, p.store, p.item = true, st, it
	p.names = p.names[:0]
	for _, c := range st.List() {
		p.names = append(p.names, c.Name)
	}
	p.cursor = 0
	p.typing = false
	p.input.SetValue("")
	p.input.Blur()
	return nil
}

func (p *Picker) close() { p.visible = false; p.typing = false; p.input.Blur() }

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
	return toastCmd("★ Added to "+name, false)
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
		p.cursor = min(p.cursor+1, len(p.names))
	case "k", "up":
		p.cursor = max(p.cursor-1, 0)
	case "enter":
		if p.cursor < len(p.names) {
			return p.add(p.names[p.cursor])
		}
		p.typing = true
		return p.input.Focus()
	}
	return nil
}

func (p Picker) View() string {
	rows := append(append([]string(nil), p.names...), newCollectionRow)
	extra, hint := "", "enter choose · j/k move · esc close"
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
}

func NewCollectionsView(pal render.Palette) CollectionsView {
	return CollectionsView{pal: pal, input: newInput("new name")}
}

func (c *CollectionsView) SetSize(w, h int) { c.w, c.h = w, h }

func (c *CollectionsView) Show(st *collections.Store) {
	c.visible, c.store, c.in, c.cursor, c.confirm, c.rename = true, st, "", 0, "", false
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
		hint = "enter open · d remove · e export · esc back"
		col, _ := c.store.Get(c.in)
		for _, it := range col.Items {
			rows = append(rows, fg.Render(it.Label)+dim.Render("  ·  "+it.Book))
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
		extra = lipgloss.NewStyle().Foreground(c.pal.Rose).Render("Remove " + ref.Display(it.Ref) + "? y/n")
	case c.confirm == "D":
		name, _ := c.selectedName()
		extra = lipgloss.NewStyle().Foreground(c.pal.Rose).Render("Delete " + name + "? y/n")
	}
	return listBox(c.pal, c.w, c.h, title, rows, c.cursor, extra, hint)
}
