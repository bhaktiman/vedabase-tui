package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/library"
	"vedabase-tui/internal/ref"
)

func (a *App) treeVisible() bool {
	return a.lib != nil && !a.state.TreeHidden && !a.state.Zen && a.w >= minTreeCols
}

func (a *App) treeW() int { return min(max(a.w*30/100, 24), 40) }

func (a *App) bodyH() int { return max(a.h-2, 1) }

// framed reports whether the panes get borders; tiny terminals go without.
// Distraction-free reading has no borders either.
func (a *App) framed() bool { return !a.state.Zen && a.w >= minFrameCols && a.h >= minFrameRows }

// bw is the border width of a pane: 1 cell each side, or 0 when unframed.
func (a *App) bw() int {
	if a.framed() {
		return 1
	}
	return 0
}

// readerOuterW is the reader pane's width with its border; the tree takes its
// share (and, unframed, a one-column divider) off it.
func (a *App) readerOuterW() int {
	switch {
	case !a.treeVisible():
		return max(a.w, 1)
	case a.framed():
		return max(a.w-a.treeW(), 1)
	}
	return max(a.w-a.treeW()-1, 1)
}

// readerW is the width of the reader's text.
func (a *App) readerW() int { return max(a.readerOuterW()-2*a.bw(), 1) }

// textW is the width the item is rendered at: reading_width unless the user
// asked for the whole pane or the pane is narrower anyway.
func (a *App) textW() int {
	if a.state.Wide || a.settings.ReadingWidth <= 0 {
		return a.readerW()
	}
	return min(a.readerW(), a.settings.ReadingWidth)
}

// colIndent is the left margin that centres the text column in the pane.
func (a *App) colIndent() int { return (a.readerW() - a.textW()) / 2 }

// treeInnerW is the width of the tree's rows.
func (a *App) treeInnerW() int { return max(a.treeW()-2*a.bw(), 1) }

// innerH is the height of the text inside a pane.
func (a *App) innerH() int { return max(a.bodyH()-2*a.bw(), 1) }

func (a *App) layout() {
	th := a.innerH()
	if a.filterShown() {
		th = max(th-1, 1)
	}
	a.tree.SetSize(a.treeInnerW(), th)
	a.reader.SetSize(a.textW(), a.innerH())
	a.search.SetSize(a.w, a.h)
	a.picker.SetSize(a.w, a.h)
	a.colview.SetSize(a.w, a.h)
}

// relayout re-renders at once when the reader's width changed.
func (a *App) relayout() tea.Cmd {
	a.layout()
	if a.hasItem && a.renderedW != a.textW() {
		return a.renderCmd()
	}
	return nil
}

// resize re-renders after the window has been quiet for a moment.
func (a *App) resize(w, h int) tea.Cmd {
	a.w, a.h = w, h
	a.focusTree = a.focusTree && a.treeVisible()
	a.layout()
	if a.hasItem && a.renderedW != a.textW() {
		a.resizeSeq++
		return a.tick(resizeQuiet, resizeTickMsg{a.resizeSeq})
	}
	return nil
}

func (a *App) View() tea.View {
	v := tea.NewView(a.content())
	v.AltScreen = true
	if a.pal.Paint {
		v.BackgroundColor, v.ForegroundColor = a.pal.Bg, a.pal.Fg
	}
	if a.settings.Mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func padTo(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func (a *App) content() string {
	if a.lib == nil {
		return a.startupView()
	}
	switch {
	case a.help:
		return a.overlayView(a.helpView())
	case a.search.visible:
		a.search.cdView = ""
		if a.cd != nil && a.cd.kind == cdSearch {
			a.search.cdView = strings.Join(a.countdownLines(a.w/2), " ")
		}
		return a.overlayView(a.search.View())
	case a.picker.visible:
		return a.overlayView(a.picker.View())
	case a.colview.visible:
		return a.overlayView(a.colview.View())
	case a.themes.visible:
		return a.overlayView(a.themeView())
	}
	body := a.reader.Lines()
	pad := strings.Repeat(" ", a.colIndent())
	for i, l := range body {
		if l != "" {
			body[i] = pad + l
		}
	}
	if a.status != "" && !a.hasItem {
		for i, l := range a.statusLines() {
			if i < len(body) {
				body[i] = pad + l
			}
		}
	}
	if a.state.Zen {
		return a.zenView(body)
	}
	if len(body) > 0 { // bottom-right of the reader
		last := len(body) - 1
		body[last] = a.placeToast(body[last], a.readerW())
	}
	var tl []string
	a.tree.focused = a.focusTree // before View, so the cursor style follows focus at once
	if a.treeVisible() {
		tl = strings.Split(a.tree.View(), "\n")
		if a.filterShown() {
			tl = append(tl, a.filterLine())
		}
	}
	if a.framed() {
		btn := ""
		if a.webButtonOn() {
			btn = webBtnLabel
		}
		var th []thumb
		if t, ok := a.readerThumb(); ok {
			th = append(th, t)
		}
		rows := a.box(body, a.readerOuterW(), a.bodyH(), a.readerTitle(), !a.focusTree, btn, "", th...)
		if a.treeVisible() {
			left := a.box(tl, a.treeW(), a.bodyH(), "Contents", a.focusTree, "", a.siteTagView(a.treeW()-2))
			for i := range rows {
				rows[i] = left[i] + rows[i]
			}
		}
		return strings.Join(append(append([]string{a.topBar()}, rows...), a.bottomBar()), "\n")
	}
	if a.treeVisible() {
		div := lipgloss.NewStyle().Foreground(a.pal.Dim).Render("│")
		for i := range body {
			left := ""
			if i < len(tl) {
				left = tl[i]
			}
			body[i] = padTo(left, a.treeW()) + div + body[i]
		}
	}
	return strings.Join(append(append([]string{a.topBar()}, body...), a.bottomBar()), "\n")
}

// zenView is the whole screen in distraction-free mode: the text column with
// a blank row above and below. The toast sits on the last row.
func (a *App) zenView(body []string) string {
	rows := append(append([]string{""}, body...), "")
	if len(rows) > a.h {
		rows = rows[:max(a.h, 1)]
	}
	last := len(rows) - 1
	rows[last] = a.placeToast(rows[last], a.w)
	if a.updCmd != "" {
		rows[last] = a.bottomBar()
	}
	return strings.Join(rows, "\n")
}

// readerTitle labels the reader's frame: the open item, or the document title.
func (a *App) readerTitle() string {
	switch {
	case a.ref == "":
		return "Reading"
	case a.hasItem && a.item.Document != nil && a.node != nil:
		return a.node.Label()
	}
	return ref.Display(a.ref)
}

// box frames lines (each at most ow-2 wide, padded here) in a rounded border
// ow x oh outer, the title set into the top edge. The focused frame is gold.
// btn, if set, sits right-aligned in the top edge; tag is centred in the
// bottom edge (already styled). th, if given, marks a run of the right edge as a scroll thumb.
func (a *App) box(lines []string, ow, oh int, title string, focused bool, btn, tag string, th ...thumb) []string {
	col, tst := a.pal.Dim, lipgloss.NewStyle().Foreground(a.pal.Dim)
	if focused {
		col = a.pal.Accent
		tst = lipgloss.NewStyle().Foreground(a.pal.Accent).Bold(true)
	}
	edge := lipgloss.NewStyle().Foreground(col)
	iw := max(ow-2, 0)
	room := max(iw-3, 0)
	if btn != "" {
		room, _ = titleRoom(ow, lipgloss.Width(title))
	}
	title = truncate(title, room)
	fill := max(iw-lipgloss.Width(title)-3, 0)
	top := edge.Render("╭─ ") + tst.Render(title) + edge.Render(" "+strings.Repeat("─", fill)+"╮")
	if btn != "" {
		label := lipgloss.NewStyle().Foreground(a.pal.Dim).Render(btn)
		if focused {
			label = gradientText(btn, stops(a.pal)...)
		}
		fill = max(ow-3-lipgloss.Width(title)-1-btnSpan, 0)
		top = edge.Render("╭─ ") + tst.Render(title) + edge.Render(" "+strings.Repeat("─", fill)+" ") +
			label + edge.Render(" ─╮")
	}
	if title == "" {
		top = edge.Render("╭" + strings.Repeat("─", iw) + "╮")
	}
	side := edge.Render("│")
	out := []string{top}
	mark := lipgloss.NewStyle().Foreground(a.pal.Fg).Render("┃") // plain, easy to spot on either border colour
	for i := 0; i < oh-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		right := side
		if len(th) > 0 && i >= th[0].start && i < th[0].start+th[0].h {
			right = mark
		}
		out = append(out, side+padTo(l, iw)+right)
	}
	if tag != "" && iw >= lipgloss.Width(tag)+4 {
		l := (iw - lipgloss.Width(tag) - 2) / 2
		r := iw - lipgloss.Width(tag) - 2 - l
		return append(out, edge.Render("╰"+strings.Repeat("─", l)+" ")+tag+
			edge.Render(" "+strings.Repeat("─", r)+"╯"))
	}
	return append(out, edge.Render("╰"+strings.Repeat("─", iw)+"╯"))
}

// overlayView centres an overlay box on the screen, toasts still at the bottom.
func (a *App) overlayView(box string) string {
	lines := strings.Split(lipgloss.Place(a.w, a.h, lipgloss.Center, lipgloss.Center, box), "\n")
	if len(lines) > 0 {
		last := len(lines) - 1
		lines[last] = a.placeToast(lines[last], a.w)
	}
	return strings.Join(lines, "\n")
}

func (a *App) startupView() string {
	var line string
	switch {
	case a.libErr != nil:
		line = lipgloss.NewStyle().Foreground(a.pal.Rose).Render("Can't reach vedabase.cc — press r to retry, q to quit")
	case a.progTotal.Load() > 0:
		line = a.startupBar(a.progDone.Load(), a.progTotal.Load())
	default:
		line = a.startupBar(a.progDone.Load(), 0)
	}
	return lipgloss.PlaceHorizontal(a.w, lipgloss.Center, line)
}

// statusLines is what replaces the document while there is none: the loader,
// the rate-limit wait, or a plain message.
func (a *App) statusLines() []string {
	w := a.textW()
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	switch {
	case a.status == loadingStatus && a.fx.Live(Spinner):
		return append([]string{a.spinnerGlyph() + " " + dim.Render(loadingStatus), ""}, a.shimmerLines(w)...)
	case a.status == loadingStatus && !a.fx.enabled:
		return []string{a.spinnerGlyph() + " " + dim.Render(loadingStatus)}
	case a.status == loadingStatus:
		return []string{dim.Render(loadingStatus)}
	}
	rose := lipgloss.NewStyle().Foreground(a.pal.Rose)
	lines := a.countdownLines(w)
	return append(lines, strings.Split(rose.Width(w).Render(a.status), "\n")...)
}

// minCrumb is the least of the breadcrumb the top bar keeps to make room for
// the chapter bar.
const minCrumb = 20

func (a *App) topBar() string {
	if a.node == nil {
		return padTo(lipgloss.NewStyle().Foreground(a.pal.Dim).Render(truncate(a.ref, a.w)), a.w)
	}
	var crumbs []string
	for n := a.node; n != nil; n = n.Parent {
		crumbs = append([]string{n.Label()}, crumbs...)
	}
	pos := ""
	if i, t := library.Position(a.node); t > 0 {
		pos = fmt.Sprintf("%d / %d", i, t)
	}
	path := strings.Join(crumbs, " › ")
	crumb := lipgloss.NewStyle().Foreground(a.pal.Bright).Bold(true)
	posSt := lipgloss.NewStyle().Foreground(a.pal.Dim)
	// The bar sits left of the counter. The breadcrumb gives way to it down
	// to minCrumb cells; if even that leaves no room, the bar is dropped.
	if i, t := library.Position(a.node); t > 0 {
		room := a.w - lipgloss.Width(pos) - 1 - chapterBarW - 1 - 1
		if room >= min(lipgloss.Width(path), minCrumb) {
			left := truncate(path, room)
			gap := max(a.w-lipgloss.Width(left)-chapterBarW-1-lipgloss.Width(pos), 1)
			return crumb.Render(left) + strings.Repeat(" ", gap) + a.chapterBar(i, t) + " " + posSt.Render(pos)
		}
	}
	left := truncate(path, max(a.w-len(pos)-2, 1))
	gap := max(a.w-lipgloss.Width(left)-lipgloss.Width(pos), 1)
	return crumb.Render(left) + strings.Repeat(" ", gap) + posSt.Render(pos)
}

// partHit is where a part label sits in the bottom bar.
type partHit struct {
	x0, x1 int // columns [x0, x1)
	key    string
}

func (a *App) bottomBar() string {
	if a.updCmd != "" { // until the next key, so it can be copied
		return lipgloss.NewStyle().Foreground(a.pal.Gold).MaxWidth(max(a.w, 1)).Render(a.updCmd)
	}
	line, _ := a.bottomLayout()
	return line
}

// focusLabel is the bottom bar's first word, in the focus colour.
func (a *App) focusLabel() string {
	name := "Reading"
	if a.focusTree && a.treeVisible() {
		name = "Contents"
	}
	return lipgloss.NewStyle().Foreground(a.pal.Accent).Bold(true).Render("● " + name)
}

// hintLine is the key hints that fit in room cells. Whole hints are dropped,
// least important first; "? help" stays as long as anything does.
func (a *App) hintLine(room int) string {
	width := "w full width"
	if a.state.Wide {
		width = "w centred"
	}
	// Most important first; dropping takes from the end.
	keep := []string{"? help", "/ search", "s save", "e export", width, "t sidebar", "T theme",
		"o web", ": go to", "h/l prev/next", "tab focus"}
	// Display order, as indexes into keep.
	order := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 0}
	for n := len(keep); n > 0; n-- {
		var out []string
		for _, i := range order {
			if i < n {
				out = append(out, keep[i])
			}
		}
		if s := strings.Join(out, " · "); lipgloss.Width(s) <= room {
			return s
		}
	}
	return ""
}

// bottomLayout is the bottom bar and the part labels' positions in it.
func (a *App) bottomLayout() (string, []partHit) {
	if a.gotoBar.visible {
		return padTo(a.gotoBar.View(a.w), a.w), nil
	}
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	on := lipgloss.NewStyle().Foreground(a.pal.Teal)
	off := dim.Strikethrough(true)
	parts := []struct {
		on   bool
		name string
	}{
		{a.parts.Devanagari, "1 dev"}, {a.parts.Verse, "2 verse"}, {a.parts.Synonyms, "3 words"},
		{a.parts.Translation, "4 trans"}, {a.parts.Purport, "5 purport"},
	}
	var ps []string
	plain := 0
	var hits []partHit
	rel := 0 // column within the right-hand block
	exW := 0
	ex := a.exportStatus()
	if ex != "" {
		exW = lipgloss.Width(ex) + 3
		rel = exW
	}
	for _, p := range parts {
		st := off
		if p.on {
			st = on
		}
		ps = append(ps, st.Render(p.name))
		hits = append(hits, partHit{rel, rel + len(p.name), p.name[:1]})
		rel += len(p.name) + 1
		plain += len(p.name) + 1
	}
	right := strings.Join(ps, " ")
	if ex != "" {
		right = lipgloss.NewStyle().Foreground(a.pal.Gold).Render(ex) + dim.Render(" · ") + right
		plain += exW
	}
	shift := func(x int) []partHit {
		for i := range hits {
			hits[i].x0 += x
			hits[i].x1 += x
		}
		return hits
	}
	ind := a.focusLabel()
	iw := lipgloss.Width(ind)
	room := a.w - plain - 2 - iw - 2
	// The update offer leads the hints, and stays when they are hidden.
	lead, leadW := "", 0
	switch {
	case a.updating:
		t := "Downloading " + a.avail + "…"
		if total := a.updTotal.Load(); total > 0 {
			t += fmt.Sprintf(" %d%%", a.updDone.Load()*100/total)
		}
		lead = lipgloss.NewStyle().Foreground(a.pal.Accent).Render(t)
	case a.avail != "" && !a.sideShowsUpdate():
		lead = lipgloss.NewStyle().Foreground(a.pal.Accent).Render("U update to " + a.avail)
	}
	if lead != "" {
		leadW = lipgloss.Width(lead)
		if room < leadW {
			lead, leadW = "", 0
		}
	}
	hints := ""
	if !a.state.HideHints && !a.updating {
		hints = a.hintLine(room - leadW - min(leadW, 3))
	}
	if room < 10 && lead == "" {
		line := ind + " " + right
		return padTo(lipgloss.NewStyle().MaxWidth(max(a.w, 1)).Render(line), a.w), shift(iw + 1)
	}
	if lead != "" && hints != "" {
		hints = dim.Render(" · ") + dim.Render(hints)
	} else {
		hints = dim.Render(hints)
	}
	left := ind + "  " + lead + hints
	gap := max(a.w-lipgloss.Width(left)-plain+1, 1)
	return left + strings.Repeat(" ", gap) + right, shift(lipgloss.Width(left) + gap)
}
