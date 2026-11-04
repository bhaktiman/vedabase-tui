package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type helpRow struct{ key, what string }

// helpGroups in display order. The first half (rounded up) is the left
// column; the order keeps the two columns near the same height, so the help
// fits a 120×30 terminal.
var helpGroups = []struct {
	name string
	rows []helpRow
}{
	{"View", []helpRow{
		{"tab", "Switch focus"}, {"t", "Hide / show sidebar"}, {"< / >", "Narrower / wider sidebar"},
		{"w", "Centred / full width"},
		{"H", "Hide / show key hints"}, {"F", "Distraction-free reading"},
	}},
	{"Tree", []helpRow{
		{"j / k", "Move down / up"}, {"h / l", "Collapse / expand"},
		{"Enter", "Open item / expand"}, {"gg / G", "First / last"},
		{"f", "Filter the tree"}, {"z", "Collapse all"},
	}},
	{"Collections", []helpRow{
		{"s", "Add to collection"}, {"c", "Jump to collection / item"},
		{"R", "Rename collection (sidebar)"},
		{"D / d", "Delete collection / remove item"},
		{"J / K", "Move item down / up"}, {"e", "Export collection"},
	}},
	{"Reader", []helpRow{
		{"j / k", "Scroll line"}, {"h / l", "Previous / next item"},
		{"Ctrl-d / Ctrl-u", "Half page"}, {"gg / G", "Top / bottom"},
		{"1 – 5", "Toggle parts"}, {"o", "Open on vedabase.cc"},
		{"R", "Re-fetch this item"}, {"y", "Copy (tick parts, y copies)"},
		{"Backspace / [ / ]", "Back after following a link / Forward"},
		{"f", "Key labels on links"}, {"Ctrl-f", "Find on the page (n / N, Esc)"},
		{"a", "Note on this item"},
	}},
	{"Overlays", []helpRow{
		{"/", "Search (ctrl-n/p to results)"}, {"n / N", "Next / previous result"},
		{":", "Go to reference"}, {"e", "Export item"},
		{"T", "Choose theme"}, {"L", "Language"}, {"U / W", "Update / what's new"},
		{"?", "This help"}, {"q / Esc", "Close overlay; quit"},
	}},
}

// helpView lists the keys, in two columns when the screen is wide enough
// (one column is taller than most terminals).
func (a *App) helpView() string {
	gold := lipgloss.NewStyle().Foreground(a.pal.Gold).Bold(true)
	key := lipgloss.NewStyle().Foreground(a.pal.Teal)
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	group := func(lines []string, i int) []string {
		g := helpGroups[i]
		lines = append(lines, "", gold.Render(g.name))
		for _, r := range g.rows {
			lines = append(lines, "  "+key.Render(r.key)+strings.Repeat(" ", max(17-lipgloss.Width(r.key), 1))+r.what)
		}
		return lines
	}
	var one, left, right []string
	for i := range helpGroups {
		one = group(one, i)
		if i < (len(helpGroups)+1)/2 {
			left = group(left, i)
		} else {
			right = group(right, i)
		}
	}
	body := strings.Join(one, "\n")
	two := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(left, "\n"), "    ", strings.Join(right, "\n"))
	if lipgloss.Width(two)+6 <= a.w { // border and padding
		body = two
	}
	lines := []string{gradientBold("Keys", stops(a.pal)...), body, "", dim.Render("any key closes")}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(a.pal.Dim).
		Padding(0, 2).Render(strings.Join(lines, "\n"))
}
